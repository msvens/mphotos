package server

import (
	"encoding/gob"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/gdrive"
	"github.com/msvens/mphotos/internal/gmail"
	"github.com/msvens/mphotos/internal/img"
	"github.com/msvens/mphotos/internal/model"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type mserver struct {
	db           model.DataStore
	ds           *gdrive.DriveService
	ms           *gmail.GmailService
	r            *mux.Router
	l            *zap.SugaredLogger
	prefixPath   string
	store        *sessions.CookieStore
	cookieName   string
	guestCookie  string
	tokenFile    string
	gconfig      *oauth2.Config
	imgDir       string
	cameraDir    string
	thumbDir     string
	portraitDir  string
	landscapeDir string
	squareDir    string
	resizeDir    string
}

func NewServer(prefixPath string) *mserver {

	s := mserver{}
	s.prefixPath = prefixPath

	//Initialize session
	authKeyOne := []byte(config.SessionAuthcKey())
	encKeyOne := []byte(config.SessionEncKey())
	s.cookieName = config.SessionCookieName()
	s.guestCookie = s.cookieName + "-guest"
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
	gob.Register(SessionGuest{})

	//setup logging
	l, _ := zap.NewDevelopment()
	s.l = l.Sugar()

	s.r = mux.NewRouter()

	var err error
	if s.db, err = model.NewDB(); err != nil {
		s.l.Panicw("could not create dbservice", "error", err)
	}

	//init img paths:
	//s.rootDir = config.ServiceRoot()
	s.imgDir = config.ServicePath("img")
	s.cameraDir = config.ServicePath("cameras")
	s.thumbDir = config.ServicePath("thumb")
	s.portraitDir = config.ServicePath("portrait")
	s.landscapeDir = config.ServicePath("landscape")
	s.squareDir = config.ServicePath("square")
	s.resizeDir = config.ServicePath("resize")
	if err = os.MkdirAll(s.imgDir, 0744); err != nil {
		s.l.Panicw("could not create img dir", zap.Error(err))
	}
	if err = img.CreateImageDir(config.ServiceRoot()); err != nil {
		s.l.Panicw("could not create image dirs", zap.Error(err))
	}
	if err = os.MkdirAll(s.imgDir, 0744); err != nil {
		s.l.Panicw("could not create img dir", zap.Error(err))
	}
	if err = os.MkdirAll(s.cameraDir, 0744); err != nil {
		s.l.Panicw("could not camera dir", zap.Error(err))
	}

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
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", gdrive.ReadOnlyScope(), gmail.ComposeScope()},
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

	if err := s.db.CloseDb(); err != nil {
		s.l.Fatalw("db close failed", zap.Error(err))
	}

	s.l.Info("server exited properly")
}

/*
func (s *mserver) setDriveService(ds *gdrive.DriveService) {
	s.ds = ds
	//s.ps.DriveSrv = s.ds
}
*/

func (s *mserver) setGoogleServices(token *oauth2.Token) error {
	if drv, err := gdrive.NewDriveService(token, s.gconfig); err != nil {
		s.l.Errorw("cannot create google drive service", zap.Error(err))
		s.ds = nil
		return err
	} else {
		s.ds = drv
	}
	if srv, err := gmail.NewGmailService(token, s.gconfig); err != nil {
		s.l.Errorw("cannot create google mail service", zap.Error(err))
		s.ms = nil
		return err
	} else {
		s.ms = srv
		return nil
	}
}

//Error Handling

//async
