package main

import (
	"flag"
	"github.com/gorilla/mux"
	"github.com/msvens/mphotos/config"
	"html/template"
	"log"
	"net/http"
)

var templates *template.Template

type Page struct {
	IsGoogleLoggedIn bool
	IsPhotosLoggedIn bool
}

func init() {
	config.InitConfig()
	templates = template.Must(template.ParseFiles("tmpl/index.html"))
}

func main() {
	var dir string

	flag.StringVar(&dir, "dir", "./static/", "the directory to serve files from. Defaults to the current dir")
	flag.Parse()

	r := mux.NewRouter()
	InitApi(r, "/api")

	r.Path("/api").HandlerFunc(handleRoot)

	//auth
	r.Path("/api/auth/login").HandlerFunc(HandleGoogleLogin)
	r.Path("/api/auth/callback").HandlerFunc(HandleGoogleCallback)

	//static files
	r.PathPrefix("/api/static/").Handler(http.StripPrefix("/api/static/", http.FileServer(http.Dir(dir))))

	//auth
	InitGoogleAuth()
	err := AuthFromFile()
	if err != nil {
		log.Println(err)
	}
	http.ListenAndServe(":8050", r)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "index.html", newPage(w, r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func newPage(w http.ResponseWriter, r *http.Request) *Page {
	return &Page{IsGoogleLoggedIn: isGoogleLoggedIn(), IsPhotosLoggedIn: isPhotosLogin(w, r)}
}
