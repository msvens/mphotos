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
	IsLoggedIn bool
}

func init() {
	config.InitConfig()
	templates = template.Must(template.ParseFiles("tmpl/index.html", "tmpl/search.html", "tmpl/setfolder.html"))
}

func main() {
	var dir string

	flag.StringVar(&dir, "dir", "./static/", "the directory to serve files from. Defaults to the current dir")
	flag.Parse()

	r := mux.NewRouter()
	InitApi(r, "/api")

	r.Path("/api").HandlerFunc(handleRoot)
	r.Path("/api/ui/search").HandlerFunc(handleSearch)
	r.Path("/api/ui/setfolder").HandlerFunc(handleSetFolder)

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

func handleSetFolder(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "setfolder.html", &Page{IsLoggedIn()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "search.html", &Page{IsLoggedIn()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "index.html", &Page{IsLoggedIn()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
