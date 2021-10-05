package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"io"
	"net/http"
	"strings"
)

type PSResponse struct {
	Err  *ApiError   `json:"error,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

const (
	//maxFormSize = 32 << 20
	contentType = "Content-Type"
	contentJson = "application/json"
)

var decoder = schema.NewDecoder()

type mHandler func(w http.ResponseWriter, r *http.Request) (interface{}, error)
type reqHandler func(r *http.Request) (interface{}, error)
type loginHandler func(r *http.Request, loggedIn bool) (interface{}, error)
type guestHandler func(r *http.Request, uuid uuid.UUID) (interface{}, error)

var loggedInCtxKey = &contextKey{"loggedIn"}
var guestCtxKey = &contextKey{"guest"}

type contextKey struct {
	name string
}

func ctxLoggedIn(ctx context.Context) bool {
	if raw, found := ctx.Value(loggedInCtxKey).(bool); found {
		return raw
	} else {
		return true
	}
}

func ctxGuest(ctx context.Context) uuid.UUID {
	if raw, found := ctx.Value(guestCtxKey).(uuid.UUID); found {
		return raw
	} else {
		return emptyuuid
	}
}

//Middleware to set information about logged in user as well as the
//current guest

func uid(r *http.Request, idname string, uuID *uuid.UUID) error {
	if id, err := uuid.Parse(Var(r, idname)); err != nil {
		return BadRequestError("Could not parse photo id")
	} else {
		*uuID = id
		return nil
	}
}

func (s *mserver) userGuestInfoMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loggedIn, loginErr := s.checkLogin(w, r)
		uuid, err := guestUUID(w, r, s)
		ctx := context.WithValue(r.Context(), loggedInCtxKey, loggedIn)
		ctx = context.WithValue(ctx, guestCtxKey, uuid)
		r = r.WithContext(ctx)
		s.l.Debugw("MWReq", "uri", r.RequestURI, "method", r.Method, "loggedIn", loggedIn, "guest", uuid, "guest error", err, "user error", loginErr)
		next.ServeHTTP(w, r)
	})
}

func (s *mserver) authOnly(rh reqHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//s.l.Debugw("AuthReq", "uri", r.RequestURI, "method", r.Method)
		if ctxLoggedIn(r.Context()) {
			data, err := rh(r)
			psResponse(data, err, w)
		} else {
			psResponse(nil, UnauthorizedError("user not logged in"), w)
		}
	}
}

func (s *mserver) mResponse(handler mHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := handler(w, r)
		psResponse(data, err, w)
	}
}

func (s *mserver) loginInfo(lh loginHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := lh(r, ctxLoggedIn(r.Context()))
		psResponse(data, err, w)
	}
}

func (s *mserver) guestOnly(gh guestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var guest = ctxGuest(r.Context())
		if guest == emptyuuid {
			psResponse(nil, UnauthorizedError("guest not found"), w)
		} else {
			data, err := gh(r, guest)
			psResponse(data, err, w)
		}
	}
}

func psResponse(data interface{}, err error, w http.ResponseWriter) {
	setJson(w)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	var resp PSResponse
	if err != nil {
		fmt.Println("request error", err)
		resp = PSResponse{ResolveError(err), nil}
	} else if data != nil {
		resp = PSResponse{nil, data}
	} else {
		resp = PSResponse{InternalError("no payload"), nil}
	}
	e := enc.Encode(resp)
	if e != nil {
		fmt.Println("could not encode response", e)
	}
}

func setJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func Var(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}

func decodeRequest(r *http.Request, dst interface{}) error {
	if strings.Contains(r.Header.Get(contentType), contentJson) {
		return decodeJson(r, dst)
	}
	err := r.ParseForm()
	if err != nil {
		return BadRequestError("could not parse form: " + err.Error())
	}
	err = decoder.Decode(dst, r.Form)
	if err != nil {
		return BadRequestError("could not decode form: " + err.Error())
	}
	return nil
}

func decodeJson(r *http.Request, dst interface{}) error {
	// Buffer the body

	//bodyBuffer, _ := ioutil.ReadAll(r.Body)
	//println(string(bodyBuffer))
	//r.Body = ioutil.NopCloser(bytes.NewReader(bodyBuffer))

	decoder := json.NewDecoder(r.Body)
	//decoder.DisallowUnknownFields()
	//We assume it is is ptr
	err := decoder.Decode(dst)
	if err != nil {
		//s.l.Infow("decode error", zap.Error(err))
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return BadRequestError(msg)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return BadRequestError("Request body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return BadRequestError(msg)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return BadRequestError(msg)

		case errors.Is(err, io.EOF):
			return BadRequestError("Request body must not be empty")

		default:
			return InternalError(err.Error())
		}
	}
	err = decoder.Decode(&struct{}{})
	if err != io.EOF {
		return BadRequestError("Request body must only contain a single JSON object")
	}
	return nil
}
