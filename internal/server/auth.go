package server

import (
	"encoding/json"
	"github.com/gorilla/sessions"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/internal/config"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"net/http"
	"os"
)

type AuthUser struct {
	Authenticated bool `json:"authenticated"`
}

//MPhotos Auth Handlers and Methods
//
//

func (s *mserver) handleLogin(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	type request struct {
		Password string `json:"password" schema:"password"`
	}
	session, err := s.store.Get(r, config.SessionCookieName())
	if err != nil {
		return nil, InternalError(err.Error())
	}
	var loginParams request
	if err = decodeRequest(r, &loginParams); err != nil {
		return nil, err
	} else if loginParams.Password != config.ServicePassword() {
		if e := session.Save(r, w); e != nil {
			return nil, InternalError(e.Error())
		}
		return nil, UnauthorizedError("Incorret user password")
	}
	user := &AuthUser{true}
	session.Values["user"] = user
	if err := session.Save(r, w); err != nil {
		return nil, InternalError(err.Error())
	}
	return user, nil
}

func (s *mserver) handleLogout(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if session, err := s.store.Get(r, config.SessionCookieName()); err != nil {
		return nil, InternalError(err.Error())
	} else {
		session.Values["user"] = AuthUser{}
		session.Options.MaxAge = -1
		if err := session.Save(r, w); err != nil {
			return nil, InternalError(err.Error())
		}
		return session.Values["user"], nil
	}
}

func (s *mserver) handleLoggedIn(r *http.Request, loggedIn bool) (interface{}, error) {
	return AuthUser{loggedIn}, nil
}

func (s *mserver) checkLogin(w http.ResponseWriter, r *http.Request) error {
	session, err := s.store.Get(r, s.cookieName)
	if err != nil {
		return InternalError(err.Error())
	}
	user := sessionUser(session)
	if !user.Authenticated {
		err = session.Save(r, w)
		if err != nil {
			return InternalError(err.Error())
		}
		return UnauthorizedError("user not authenticated to api")
	}
	return nil
}

func (s *mserver) isLoggedIn(w http.ResponseWriter, r *http.Request) bool {
	if err := s.checkLogin(w, r); err == nil {
		return true
	} else {
		return false
	}
}

func sessionUser(s *sessions.Session) AuthUser {
	val := s.Values["user"]
	var user = AuthUser{}
	user, ok := val.(AuthUser)
	if !ok {
		return AuthUser{Authenticated: false}
	}
	return user
}

// GOOGLE Auth methods and handler below:
//
//

// Retrieves a token from a local file.
func (s *mserver) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func (s *mserver) authFromFile() error {
	if token, err := s.tokenFromFile(s.tokenFile); err != nil {
		s.setDriveService(nil)
		return err
	} else {
		if drv, err := mdrive.NewDriveService(token, s.gconfig); err != nil {
			s.setDriveService(nil)
			return err
		} else {
			s.setDriveService(drv)
			return nil
		}
	}
}

func (s *mserver) getToken(r *http.Request) (*oauth2.Token, error) {
	state := r.FormValue("state")
	code := r.FormValue("code")
	if state != "state-token" {
		return nil, BadRequestError("invalid oauth state")
	}
	if token, err := s.gconfig.Exchange(context.TODO(), code); err != nil {
		s.l.Errorw("code exchage error", zap.Error(err))
		return nil, UnauthorizedError(err.Error())
	} else {
		return token, nil
	}

}

func (s *mserver) saveToken(path string, token *oauth2.Token) {
	s.l.Info("saving credential file", "path", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		s.l.Errorw("unable to open tokenfile", zap.Error(err))
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (s *mserver) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if token, err := s.getToken(r); err != nil {
		s.l.Errorw("", zap.Error(err))
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	} else {
		s.saveToken(s.tokenFile, token)
		if drv, err := mdrive.NewDriveService(token, s.gconfig); err != nil {
			s.l.Errorw("cannot create mdrive service", zap.Error(err))
		} else {
			s.setDriveService(drv)
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		}
	}
}

func (s *mserver) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	token, err := s.tokenFromFile(s.tokenFile)
	if err != nil {
		s.l.Info("could not token from file, redirect to google")
		url := s.gconfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	} else {
		s.l.Info("read token from file")
		//gtoken = token
		drv, err := mdrive.NewDriveService(token, s.gconfig)
		if err != nil {
			s.l.Errorw("could not create mdrive service", zap.Error(err))
		}
		s.setDriveService(drv)
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
	}
}

func (s *mserver) isGoogleConnected() bool {
	if s.ds == nil {
		return false
	} else if _, err := s.ds.Get(s.ds.Root.Id); err != nil {
		s.l.Errorw("could not retrieve root folder", zap.Error(err))
		return false
	}
	return true
}
