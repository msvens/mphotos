package server

import (
	"encoding/gob"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/spf13/viper"
)

func setConfig(t *testing.T, key string, value interface{}) {
	t.Helper()
	old := viper.Get(key)
	viper.Set(key, value)
	t.Cleanup(func() { viper.Set(key, old) })
}

func loginReq(password string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/login",
		strings.NewReader(`{"password":"`+password+`"}`))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestHandleLoginPassword(t *testing.T) {
	gob.Register(AuthUser{}) // newServer does this, but it also needs a DB
	s := &mserver{
		store:      sessions.NewCookieStore([]byte("test-auth-key"), []byte("0123456789abcdef")),
		cookieName: "test",
	}
	setConfig(t, "auth.method", "password")

	// An unconfigured password must never match, not even an empty login.
	setConfig(t, "auth.password", "")
	if _, err := s.handleLogin(httptest.NewRecorder(), loginReq("")); err == nil {
		t.Error("empty configured password: expected login to be refused")
	}

	setConfig(t, "auth.password", "secret")
	if _, err := s.handleLogin(httptest.NewRecorder(), loginReq("wrong")); err == nil {
		t.Error("wrong password: expected an error")
	} else if apiErr, ok := err.(*ApiError); !ok || apiErr.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: expected a 401 ApiError, got %v", err)
	}
	if _, err := s.handleLogin(httptest.NewRecorder(), loginReq("secret")); err != nil {
		t.Errorf("right password: expected login, got %v", err)
	}
}
