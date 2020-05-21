package main

import (
	"encoding/json"
	"fmt"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/config"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"log"
	"net/http"
	"os"
)

var (
	gconfig   *oauth2.Config
	tokenFile string
)

func InitGoogleAuth() {

	tokenFile = config.ServicePath("token.json")
	//fmt.Println("this is token file: " + tokenFile + " " + config.DbName())
	//ClientId/Secret will be moved out from this file

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
	token, err := TokenFromFile(tokenFile)
	if err != nil {
		return err
	}
	//gtoken = token
	drv, err := mdrive.NewDriveService(token, gconfig)
	if err != nil {
		return err
	}
	SetPhotoService(drv)
	return err
}

func SaveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
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
		fmt.Println("read token from file")
		//gtoken = token
		drv, err := mdrive.NewDriveService(token, gconfig)
		if err != nil {
			fmt.Println(err)
		}
		SetPhotoService(drv)
		http.Redirect(w, r, "/api", http.StatusTemporaryRedirect)
	}

}

func HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	//content, err := getUserInfo(r)
	token, err := GetToken(r)
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/api", http.StatusTemporaryRedirect)
		return
	}
	SaveToken(tokenFile, token)
	//gtoken = token
	drv, err := mdrive.NewDriveService(token, gconfig)
	if err != nil {
		fmt.Println(err)
	}
	SetPhotoService(drv)
	http.Redirect(w, r, "/api", http.StatusTemporaryRedirect)
	return
}

func GetToken(r *http.Request) (*oauth2.Token, error) {
	state := r.FormValue("state")
	code := r.FormValue("code")
	if state != "state-token" {
		return nil, fmt.Errorf("invalid oauth state: %s", state)
	}
	token, err := gconfig.Exchange(context.TODO(), code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}

	return token, nil
}
