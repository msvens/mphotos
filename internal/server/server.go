package server

import (
	"database/sql"
	"encoding/gob"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mdrive"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/model"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type mserver struct {
	db         model.DataStore
	ds         *mdrive.DriveService
	r          *mux.Router
	l          *zap.SugaredLogger
	prefixPath string
	store      *sessions.CookieStore
	cookieName string
	tokenFile  string
	gconfig    *oauth2.Config
	imgDir     string
	thumbDir   string
}

func NewServer(prefixPath string) *mserver {

	s := mserver{}
	s.prefixPath = prefixPath

	//Initialize session
	authKeyOne := []byte(config.SessionAuthcKey())
	encKeyOne := []byte(config.SessionEncKey())
	s.cookieName = config.SessionCookieName()
	s.store = sessions.NewCookieStore(
		authKeyOne,
		encKeyOne,
	)
	s.store.Options = &sessions.Options{
		Path:     "/api",
		MaxAge:   60 * 60 * 24,
		HttpOnly: true,
	}
	gob.Register(AuthUser{})

	//setup logging
	l, _ := zap.NewDevelopment()
	s.l = l.Sugar()

	s.r = mux.NewRouter()

	var err error
	if s.db, err = model.NewDB(); err != nil {
		s.l.Panicw("could not create dbservice", "error", err)
	}

	//init image paths:
	//s.rootDir = config.ServiceRoot()
	s.imgDir = config.ServicePath("img")
	s.thumbDir = config.ServicePath("thumb")
	if err = os.MkdirAll(s.imgDir, 0744); err != nil {
		s.l.Panicw("could not create image dir", zap.Error(err))
	}
	if err = os.MkdirAll(s.thumbDir, 0744); err != nil {
		s.l.Panicw("could not rcreae thumb dir", zap.Error(err))
	}

	//ps.DriveSrv = driveSrv
	//ps.folderPath = ps.rootDir + "/" + folderFileName

	//ps, _ := service.NewPhotosService(s.db)
	//s.ps = ps

	//start async job channel:
	wg.Add(1)
	go worker(jobChan)

	//init google auth:
	s.tokenFile = config.ServicePath("token.json")

	s.gconfig = &oauth2.Config{
		ClientID:     config.GoogleClientId(),
		ClientSecret: config.GoogleClientSecret(),
		Endpoint:     google.Endpoint,
		RedirectURL:  config.GoogleRedirectUrl(),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", mdrive.ReadOnlyScope()},
	}

	return &s
}

func StartMServer() {
	config.InitConfig()

	s := NewServer("/api")
	s.routes()
	defer s.l.Sync()

	//auth
	err := s.authFromFile()
	if err != nil {
		s.l.Errorw("google auth", zap.Error(err))
	}

	srv := &http.Server{
		Addr:    ":8050",
		Handler: s.r,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.l.Fatalw("listen", zap.Error(err))
		}
	}()

	s.l.Info("server started")

	<-done //wait for shutdown interrupt, e.g ctrl-c

	s.l.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	close(jobChan)
	wg.Wait()
	//if s.ps != nil {
	//	s.ps.Shutdown()
	//}

	if err := srv.Shutdown(ctx); err != nil {
		s.l.Fatalw("server shutdown failed", zap.Error(err))
	}
	s.l.Info("server exited properly")
}

func (s *mserver) setDriveService(ds *mdrive.DriveService) {
	s.ds = ds
	//s.ps.DriveSrv = s.ds
}

//Error Handling
type ApiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("code: %d message: %s", e.Code, e.Message)
}

func newError(code int, message string) *ApiError {
	return &ApiError{Code: code, Message: message}
}

func UnauthorizedError(message string) *ApiError {
	return newError(http.StatusUnauthorized, message)
}

func NotFoundError(message string) *ApiError {
	return newError(http.StatusNotFound, message)
}

func BadRequestError(message string) *ApiError {
	return newError(http.StatusBadRequest, message)
}

func InternalError(message string) *ApiError {
	return newError(http.StatusInternalServerError, message)
}

func ResolveError(err error) *ApiError {
	//check for api error
	e, ok := err.(*ApiError)
	if ok {
		return e
	}
	//check for google error
	e1, ok := err.(*googleapi.Error)
	if ok {
		return &ApiError{e1.Code, e1.Message}
	}

	if err == sql.ErrNoRows {
		return NotFoundError("No such data")
	}
	//check for db error
	return InternalError(err.Error())
}

//async
