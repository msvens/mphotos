package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/service"
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

type EditPhoto struct {
	Id          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
}

func SetPhotoService(drvService *mdrive.DriveService) {
	if s, err := service.NewPhotosService(drvService); err != nil {
		logger.Panicw("could not create photos service", zap.Error(err))
	} else {
		ps = s
	}
}

/*******************Response and Request writing and parsing********************/
func decodeJson(r *http.Request, dst interface{}) error {
	if !strings.Contains(r.Header.Get(contentType), contentJson) {
		return service.BadRequestError("wrong content type: " + r.Header.Get(contentType))
	}
	//no need to set maxBodySize...should be taken care of by proxyserver
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&dst)
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

// ah decorates a function with session checks and outputs a PSResponse.
// Should be used for any function that you need to be logged in to the api for
func ah(f ReqHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debugw("AReq", "uri", r.RequestURI, "method", r.Method)
		if checkAndWrite(w, r) {
			data, err := f(r)
			psResponse(data, err, w)
		}
	}
}

// h decorates a function and outputs a PSResponse
func h(f ReqHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debugw("Req", "uri", r.RequestURI, "method", r.Method)
		data, err := f(r)
		psResponse(data, err, w)
	}
}

// hw decorates a function and outputs a PSResponse
func hw(f ReqRespHandler) HttpHandler {
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
	}
	return false
}

// Checks if a google drive connection has been established
func isGoogleConnected() bool {
	//TODO: need a proper check for this
	return ps != nil
}
