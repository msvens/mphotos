package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/msvens/mphotos/internal/config"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const oauthLoginStateMaxAge = 600 // 10 minutes

type AuthUser struct {
	Authenticated bool `json:"authenticated"`
}

func (s *mserver) handleLogin(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if config.AuthMethod() != "password" {
		return nil, newError(http.StatusMethodNotAllowed, "password login disabled; use /api/login/google")
	}
	type request struct {
		Password string `json:"password" schema:"password"`
	}
	session, _ := s.store.Get(r, s.cookieName)
	var loginParams request
	if err := decodeRequest(r, &loginParams); err != nil {
		return nil, err
	} else if loginParams.Password != config.AuthPassword() {
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

func (s *mserver) handleAuthMethod(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return map[string]string{"method": config.AuthMethod()}, nil
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
	defer func() { _ = f.Close() }()
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
	defer func() { _ = f.Close() }()
	if err := json.NewEncoder(f).Encode(token); err != nil {
		s.l.Errorw("unable to encode token", zap.Error(err))
	}
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
		s.psResponse(nil, UnauthorizedError("user not logged in"), w)
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

// Google login flow (separate from the Drive OAuth flow). Uses gconfigLogin
// which only requests openid/email/profile scopes, and verifies the returned
// email against config.AuthAllowedEmails before issuing the session cookie.

func (s *mserver) loginStateCookieName() string {
	return s.cookieName + "-login-state"
}

func (s *mserver) handleGoogleLoginRedirect(w http.ResponseWriter, r *http.Request) {
	state, err := generateOAuthState()
	if err != nil {
		s.l.Errorw("could not generate oauth state", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	stateSession, _ := s.store.Get(r, s.loginStateCookieName())
	stateSession.Values["state"] = state
	stateSession.Options.MaxAge = oauthLoginStateMaxAge
	if err := stateSession.Save(r, w); err != nil {
		s.l.Errorw("could not save oauth state cookie", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, s.gconfigLogin.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func (s *mserver) handleGoogleLoginCallback(w http.ResponseWriter, r *http.Request) {
	redirTo := func(errCode string) {
		target := config.AuthGoogleUIRedirectPath()
		if errCode != "" {
			sep := "?"
			if strings.Contains(target, "?") {
				sep = "&"
			}
			target = target + sep + "error=" + url.QueryEscape(errCode)
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}

	queryState := r.FormValue("state")
	stateSession, _ := s.store.Get(r, s.loginStateCookieName())
	expected, _ := stateSession.Values["state"].(string)
	// Always expire the state cookie after first use, regardless of outcome.
	stateSession.Options.MaxAge = -1
	_ = stateSession.Save(r, w)

	if queryState == "" || expected == "" || queryState != expected {
		s.l.Info("oauth login state mismatch")
		redirTo("invalid_state")
		return
	}

	code := r.FormValue("code")
	if code == "" {
		redirTo("invalid_state")
		return
	}

	token, err := s.gconfigLogin.Exchange(r.Context(), code)
	if err != nil {
		s.l.Errorw("login code exchange failed", zap.Error(err))
		redirTo("exchange_failed")
		return
	}

	email, verified, err := fetchGoogleUserinfo(r.Context(), token)
	if err != nil {
		s.l.Errorw("userinfo fetch failed", zap.Error(err))
		redirTo("exchange_failed")
		return
	}
	if !verified {
		s.l.Infow("google email not verified", "email", email)
		redirTo("unauthorized_email")
		return
	}
	if !emailAllowed(email, config.AuthAllowedEmails()) {
		s.l.Infow("email not in allowlist", "email", email)
		redirTo("unauthorized_email")
		return
	}

	session, _ := s.store.Get(r, s.cookieName)
	session.Values["user"] = &AuthUser{true}
	session.Options.MaxAge = Session_Month
	if err := session.Save(r, w); err != nil {
		s.l.Errorw("could not save login session", zap.Error(err))
		redirTo("exchange_failed")
		return
	}
	redirTo("")
}

func generateOAuthState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func fetchGoogleUserinfo(ctx context.Context, token *oauth2.Token) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("userinfo: status %d", resp.StatusCode)
	}
	var info struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", false, err
	}
	return info.Email, info.EmailVerified, nil
}

func emailAllowed(email string, allowlist []string) bool {
	if email == "" {
		return false
	}
	e := strings.ToLower(email)
	for _, a := range allowlist {
		if strings.ToLower(a) == e {
			return true
		}
	}
	return false
}
