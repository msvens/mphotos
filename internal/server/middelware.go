package server

import (
	"encoding/json"
	"errors"
	"fmt"
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

func (s *mserver) authOnly(rh reqHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.l.Debugw("AuthReq", "uri", r.RequestURI, "method", r.Method)
		if s.checkAndWrite(w, r) {
			data, err := rh(r)
			psResponse(data, err, w)
		}
	}
}

func (s *mserver) mResponse(handler mHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.l.Debugw("Req", "uri", r.RequestURI, "method", r.Method)
		data, err := handler(w, r)
		psResponse(data, err, w)
	}
}

func (s *mserver) loginInfo(lh loginHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var loggedIn = s.isLoggedIn(w, r)
		s.l.Debugw("LoginReq", "uri", r.RequestURI, "method", r.Method, "loggedin", loggedIn)
		data, err := lh(r, loggedIn)
		psResponse(data, err, w)
	}
}

func (s *mserver) checkAndWrite(w http.ResponseWriter, r *http.Request) bool {
	if err := s.checkLogin(w, r); err != nil {
		psResponse(nil, err, w)
		return false
	}
	return true
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
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
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
