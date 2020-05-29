package main

import (
	"encoding/json"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/config"
	"github.com/msvens/mphotos/service"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"net/http"
	"os"
)

var (
	gconfig   *oauth2.Config
	tokenFile string
)

func InitGoogleAuth() {

	tokenFile = config.ServicePath("token.json")

	gconfig = &oauth2.Config{
		ClientID:     config.GoogleClientId(),
		ClientSecret: config.GoogleClientSecret(),
		Endpoint:     google.Endpoint,
		RedirectURL:  config.GoogleRedirectUrl(),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", mdrive.ReadOnlyScope()},
	}
}

// Retrieves a token from a local file.
func TokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func AuthFromFile() error {
	if token, err := TokenFromFile(tokenFile); err != nil {
		return err
	} else {
		if drv, err := mdrive.NewDriveService(token, gconfig); err != nil {
			return err
		} else {
			SetPhotoService(drv)
			return nil
		}
	}
}

func SaveToken(path string, token *oauth2.Token) {
	logger.Info("saving credential file", "path", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		logger.Errorw("unable to open tokenfile", zap.Error(err))
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// /login
func HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	token, err := TokenFromFile(tokenFile)
	if err != nil {
		url := gconfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	} else {
		logger.Info("read token from file")
		//gtoken = token
		drv, err := mdrive.NewDriveService(token, gconfig)
		if err != nil {
			logger.Errorw("could not create mdrive service", zap.Error(err))
		}
		SetPhotoService(drv)
		http.Redirect(w, r, "/api", http.StatusTemporaryRedirect)
	}

}

func HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if token, err := GetToken(r); err != nil {
		logger.Errorw("", zap.Error(err))
		http.Redirect(w, r, "/api", http.StatusTemporaryRedirect)
		return
	} else {
		SaveToken(tokenFile, token)
		if drv, err := mdrive.NewDriveService(token, gconfig); err != nil {
			logger.Errorw("cannot create mdrive service", zap.Error(err))
		} else {
			SetPhotoService(drv)
			http.Redirect(w, r, "/api", http.StatusTemporaryRedirect)
		}
	}
}

func GetToken(r *http.Request) (*oauth2.Token, error) {
	state := r.FormValue("state")
	code := r.FormValue("code")
	if state != "state-token" {
		return nil, service.NewError(service.ApiErrorBadRequest, "invalid oauth state")
	}
	if token, err := gconfig.Exchange(context.TODO(), code); err != nil {
		logger.Errorw("code exchage error", zap.Error(err))
		return nil, service.NewError(service.ApiErrorInvalidCredentials, err.Error())
	} else {
		return token, nil
	}

}
