package main

import (
	"flag"
	"github.com/gorilla/mux"
	"github.com/msvens/mphotos/config"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var templates *template.Template
var logger *zap.SugaredLogger

//var srv *http.Server

type Page struct {
	IsGoogleLoggedIn bool
	IsPhotosLoggedIn bool
}

func init() {
	config.InitConfig()
	templates = template.Must(template.ParseFiles("tmpl/index.html"))
}

func main() {

	l, _ := zap.NewDevelopment()
	logger = l.Sugar()
	defer logger.Sync()

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
		logger.Errorw("google auth", zap.Error(err))
	}

	srv := &http.Server{
		Addr:    ":8050",
		Handler: r,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalw("listen", zap.Error(err))
		}
	}()

	logger.Info("server started")

	<-done //wait for shutdown interrupt, e.g ctrl-c

	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if ps != nil {
		ps.Shutdown()
	}

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalw("server shutdown failed", zap.Error(err))
	}
	logger.Info("server exited properly")

}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "index.html", newPage(w, r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func newPage(w http.ResponseWriter, r *http.Request) *Page {
	return &Page{IsGoogleLoggedIn: isGoogleConnected(), IsPhotosLoggedIn: isLoggedIn(w, r)}
}
