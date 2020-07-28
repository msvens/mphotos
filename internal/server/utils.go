package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/gorilla/sessions"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/internal/service"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	maxFormSize = 32 << 20
	contentType = "Content-Type"
	contentJson = "application/json"
)

//Defined http handlers
type HttpHandler func(http.ResponseWriter, *http.Request)
type ReqHandler func(r *http.Request) (interface{}, error)
type RLoginHandler func(r *http.Request, loggedIn bool) (interface{}, error)
type ReqRespHandler func(w http.ResponseWriter, r *http.Request) (interface{}, error)

// The general api json response containing either an error or data
type PSResponse struct {
	Err  *service.ApiError `json:"error,omitempty"`
	Data interface{}       `json:"data,omitempty"`
}

// True if the current user is authenticated
type AuthUser struct {
	Authenticated bool `json:"authenticated"`
}

func SetPhotoService(drvService *mdrive.DriveService) {
	if ps != nil {
		logger.Info("PhotoService Already existed only setting drvService")
		ps.DriveSrv = drvService
		return
	}
	if s, err := service.NewPhotosService(drvService); err != nil {
		logger.Panicw("could not create photos service", zap.Error(err))
	} else {
		ps = s
	}
}

/*******************Response and Request writing and parsing********************/
var decoder = schema.NewDecoder()

// decodeRequest extracts the query string, form or json post as a go struct.
// if the content type is Json it will try to decode post data as Json otherwise
// it will use gorilla/schema to decode the PostForm
func decodeRequest(r *http.Request, dst interface{}) error {
	if strings.Contains(r.Header.Get(contentType), contentJson) {
		return decodeJson(r, dst)
	}
	err := r.ParseForm()
	if err != nil {
		return service.BadRequestError("could not parse form: " + err.Error())
	}
	err = decoder.Decode(dst, r.Form)
	if err != nil {
		return service.BadRequestError("could not decode form: " + err.Error())
	}
	return nil
}

func decodeJson(r *http.Request, dst interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	//We assume it is is ptr
	err := decoder.Decode(dst)
	if err != nil {
		logger.Infow("decode error", zap.Error(err))
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return service.BadRequestError(msg)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return service.BadRequestError("Request body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return service.BadRequestError(msg)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return service.BadRequestError(msg)

		case errors.Is(err, io.EOF):
			return service.BadRequestError("Request body must not be empty")

		default:
			return service.InternalError(err.Error())
		}
	}
	err = decoder.Decode(&struct{}{})
	if err != io.EOF {
		return service.BadRequestError("Request body must only contain a single JSON object")
	}
	return nil
}

func Var(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}

func QPInt(r *http.Request, param string, val int) int {
	if v, f := r.URL.Query()[param]; f {
		if i, e := strconv.Atoi(v[0]); e != nil {
			logger.Errorw("failed to parse int", "value", v)
			return val
		} else {
			return i
		}
	} else {
		return val
	}
}

func QPBool(r *http.Request, param string, val bool) bool {
	if v, f := r.URL.Query()[param]; f {
		if b, e := strconv.ParseBool(v[0]); e != nil {
			logger.Errorw("Failed to parse bool", "value", v)
			return val
		} else {
			return b
		}
	} else {
		return val
	}
}

func form(r *http.Request) {
	_ = r.ParseMultipartForm(maxFormSize)
}

func setJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

// writes a PSResponse. Either writes an error or data (error is checked first)
func psResponse(data interface{}, err error, w http.ResponseWriter) {
	setJson(w)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	var resp PSResponse
	if err != nil {
		logger.Info("request error", zap.Error(err))
		resp = PSResponse{service.ResolveError(err), nil}
	} else if data != nil {
		//TODO: Check if this check is ok
		resp = PSResponse{nil, data}
	} else {
		resp = PSResponse{service.InternalError("no payload"), nil}
	}
	e := enc.Encode(resp)
	if e != nil {
		logger.Errorw("could not encode response", zap.Error(e))
	}
}

// arh (AuthenticatedRequestHandler) will return an error response if
// the user is not authenticated.
func arh(f ReqHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debugw("AReq", "uri", r.RequestURI, "method", r.Method)
		if checkAndWrite(w, r) {
			data, err := f(r)
			psResponse(data, err, w)
		}
	}
}

// lrh (LoggedInRequestHandler) will extract login information before processing
// the request
func lrh(f RLoginHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		var loggedIn = isLoggedIn(w, r)
		logger.Debugw("LReq", "uri", r.RequestURI, "method", r.Method, "loggedin", loggedIn)
		data, err := f(r, loggedIn)
		psResponse(data, err, w)
	}
}

// rh (RequestHandler) will disregard any authentication information
func rh(f ReqHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debugw("Req", "uri", r.RequestURI, "method", r.Method)
		data, err := f(r)
		psResponse(data, err, w)
	}
}

// rwh (RequestWriterHandlder) decorates a function and outputs a PSResponse
func rwh(f ReqRespHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debugw("Req", "uri", r.RequestURI, "method", r.Method)
		data, err := f(w, r)
		psResponse(data, err, w)
	}
}

/*********LOGIN CHECKING******************************/
func getSessionUser(s *sessions.Session) AuthUser {
	val := s.Values["user"]
	var user = AuthUser{}
	user, ok := val.(AuthUser)
	if !ok {
		return AuthUser{Authenticated: false}
	}
	return user
}

// Returns an error (InvalidCredentials) if a user is not logged in
func checkLogin(w http.ResponseWriter, r *http.Request) error {
	session, err := store.Get(r, cookieName)
	if err != nil {
		return service.InternalError(err.Error())
	}
	user := getSessionUser(session)
	if !user.Authenticated {
		err = session.Save(r, w)
		if err != nil {
			return service.InternalError(err.Error())
		}
		return service.UnauthorizedError("user not authenticated to api")
	}
	return nil
}

// Writes a PSResponse error if user is not logged in and returns login status
func checkAndWrite(w http.ResponseWriter, r *http.Request) bool {
	if err := checkLogin(w, r); err != nil {
		psResponse(nil, err, w)
		return false
	}
	return true
}

// Check if the user is logged in to the photo service
func isLoggedIn(w http.ResponseWriter, r *http.Request) bool {
	if err := checkLogin(w, r); err == nil {
		return true
	} else {
		return false
	}
}

// Checks if a google drive connection has been established
func isGoogleConnected() bool {
	if ps.DriveSrv == nil {
		return false
	} else if _, err := ps.DriveSrv.Get(ps.DriveSrv.Root.Id); err != nil {
		logger.Errorw("could not retrieve root folder", zap.Error(err))
		return false
	}
	return true
}
