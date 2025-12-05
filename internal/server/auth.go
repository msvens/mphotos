package server

import (
	"encoding/json"
	"github.com/msvens/mphotos/internal/config"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"os"
)

type AuthUser struct {
	Authenticated bool `json:"authenticated"`
}

func (s *mserver) handleLogin(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	type request struct {
		Password string `json:"password" schema:"password"`
	}
	//session, err := s.store.Get(r, config.SessionCookieName())
	session, _ := s.store.Get(r, s.cookieName)
	/*if err != nil {
		return nil, InternalError(err.Error())
	}*/
	var loginParams request
	if err := decodeRequest(r, &loginParams); err != nil {
		return nil, err
	} else if loginParams.Password != config.ServicePassword() {
		if e := session.Save(r, w); e != nil {
			return nil, InternalError(e.Error())
		}
		return nil, UnauthorizedError("Incorret user password")
	}
	user := &AuthUser{true}
	session.Values["user"] = user
	session.Options.MaxAge = Session_Month
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

func (s *mserver) checkLogin(w http.ResponseWriter, r *http.Request) (bool, error) {
	session, err := s.store.Get(r, s.cookieName)
	if err != nil {
		return false, InternalError(err.Error())
	}
	if val, ok := session.Values["user"]; ok {
		if user, ok := val.(AuthUser); ok {
			return user.Authenticated, nil
		} else {
			return false, InternalError("could not parse mphotos cookie value")
		}
	} else {
		return false, nil
	}
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
		//s.setDriveService(nil)
		s.ds = nil
		s.ms = nil
		return err
	} else {
		return s.setGoogleServices(token)
		/*if drv, err := gdrive.NewDriveService(token, s.gconfig); err != nil {
			s.setDriveService(nil)
			return err
		} else {
			s.setDriveService(drv)
			return nil
		}*/
	}
}

func (s *mserver) getToken(r *http.Request) (*oauth2.Token, string, error) {
	state := r.FormValue("state")
	code := r.FormValue("code")
	if state == "" {
		return nil, "", BadRequestError("invalid oauth state")
	}

	if token, err := s.gconfig.Exchange(context.TODO(), code); err != nil {
		s.l.Errorw("code exchage error", zap.Error(err))
		return nil, state, UnauthorizedError(err.Error())
	} else {
		if u, e := url.QueryUnescape(state); e != nil {
			s.l.Errorw("could not unescape state", zap.Error(e))
			return nil, state, BadRequestError("invalid ouath state")
		} else {
			return token, u, nil
		}
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
	if token, redir, err := s.getToken(r); err != nil {
		s.l.Errorw("", zap.Error(err))
		http.Redirect(w, r, redir, http.StatusTemporaryRedirect)
		return
	} else {
		s.saveToken(s.tokenFile, token)
		if err := s.setGoogleServices(token); err == nil {
			http.Redirect(w, r, redir, http.StatusTemporaryRedirect)
		}
	}
}

func (s *mserver) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	parseDir := func() (string, string) {
		root := url.QueryEscape("/")
		redir := r.URL.Query().Get("redir")
		if redir == "" {
			return root, "/"
		}
		unesc, err := url.QueryUnescape(redir)
		if err != nil {
			s.l.Info("could not unescape redirection url:  ", err)
			return root, "/"
		}
		parsed, err := url.Parse(unesc)
		if err != nil {
			s.l.Info("could not parse redirection url:  ", err)
			return root, "/"
		} else if parsed.IsAbs() {
			s.l.Info("Absolut url not allowed: ", unesc)
			return root, "/"
		} else {
			return redir, unesc
		}
	}
	//make sure only logged in users can execute this
	if !ctxLoggedIn(r.Context()) {
		psResponse(nil, UnauthorizedError("user not logged in"), w)
		return
	}
	redirUrl, unesc := parseDir()
	token, err := s.tokenFromFile(s.tokenFile)
	if err != nil {
		s.l.Info("could not read token from file, redirect to google")
		u := s.gconfig.AuthCodeURL(redirUrl, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		http.Redirect(w, r, u, http.StatusTemporaryRedirect)
	} else {
		s.l.Info("read token from file")
		if err = s.setGoogleServices(token); err == nil {
			http.Redirect(w, r, unesc, http.StatusTemporaryRedirect)
		} else {
			s.l.Info("Token not valid...trying to get a new one from google")
			u := s.gconfig.AuthCodeURL(redirUrl, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
			http.Redirect(w, r, u, http.StatusTemporaryRedirect)
		}
	}
}

func (s *mserver) isGoogleConnected() bool {
	if s.ds == nil {
		return false
	} else if err := s.ds.Check(); err != nil {
		s.l.Errorw("Drive Service Check failed", zap.Error(err))
		return false
	}
	return true
}
