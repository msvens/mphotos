package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mphotos/service"
	"go.uber.org/zap"
	"net/http"
)

const (
	maxFormSize = 32 << 20
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

/*******************Response and Request writing and parsing********************/
func Var(r *http.Request, name string) string {
	return mux.Vars(r)[name]
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
		resp = PSResponse{service.NewBackendError("no payload"), nil}
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
		if checkAndWrite(w, r) {
			data, err := f(r)
			psResponse(data, err, w)
		}
	}
}

// h decorates a function and outputs a PSResponse
func h(f ReqHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := f(r)
		psResponse(data, err, w)
	}
}

// hw decorates a function and outputs a PSResponse
func hw(f ReqRespHandler) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
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
		return service.NewError(service.ApiErrorBackendError, err.Error())
	}
	user := getSessionUser(session)
	if !user.Authenticated {
		err = session.Save(r, w)
		if err != nil {
			return service.NewError(service.ApiErrorBackendError, err.Error())
		}
		return service.NewError(service.ApiErrorInvalidCredentials, "user not authenticated to api")
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
