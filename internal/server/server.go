package server

import (
    "fmt"
	"encoding/gob"
    "github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/msvens/mphotos/internal/gdrive"
	"github.com/msvens/mphotos/internal/gmail"
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
	//dbold           model.DataStore
	pg          *dao.PGDB
	ds          *gdrive.DriveService
	ms          *gmail.GmailService
	r           *mux.Router
	l           *zap.SugaredLogger
	prefixPath  string
	store       *sessions.CookieStore
	cookieName  string
	guestCookie string
	tokenFile   string
	gconfig     *oauth2.Config
	/*imgDir       string
	cameraDir    string
	thumbDir     string
	portraitDir  string
	landscapeDir string
	squareDir    string
	resizeDir    string*/
}

func newServer(prefixPath string, logger *zap.SugaredLogger) *mserver {

	s := mserver{}
    s.l = logger
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


	s.r = mux.NewRouter()

	var err error

	if s.pg, err = dao.NewPGDB(); err != nil {
		s.l.Panicw("could not create pgdb service", zap.Error(err))
	}

	if err = dao.CreateImageDirs(); err != nil {
		s.l.Panicw("could not create img dirs", zap.Error(err))
	}

	if err = os.MkdirAll(config.CameraPath(), 0744); err != nil {
		s.l.Panicw("could not create camera dir", zap.Error(err))
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
    //setup logging
    l, err := zap.NewDevelopment()
    if err != nil {
        fmt.Println("Could not create logger exiting: ", err)
        os.Exit(1)
    }
    defer func() {
        _ = l.Sync()
    }()

    //init config
    err = config.InitConfig()
    if err != nil {
        l.Sugar().Panicw("Could not init config", zap.Error(err))
    }

	s := newServer("/api", l.Sugar())
	s.routes()

	//auth
	err = s.authFromFile()
	if err != nil {
        s.l.Infow("auth from file", zap.Error(err))
	}

	srv := &http.Server{
		Addr:    config.ServerAddr(),
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

	/*if err := s.dbold.CloseDb(); err != nil {
		s.l.Fatalw("db close failed", zap.Error(err))
	}*/

	if err := s.pg.Close(); err != nil {
		s.l.Fatalw("PGDB close failed", zap.Error(err))
	}

	s.l.Info("server exited properly")
}

func (s *mserver) setGoogleServices(token *oauth2.Token) error {
	if drv, err := gdrive.NewDriveService(token, s.gconfig); err != nil {
		s.l.Infow("cannot create google drive service", zap.Error(err))
		s.ds = nil
		return err
	} else {
		s.ds = drv
	}
	if srv, err := gmail.NewGmailService(token, s.gconfig); err != nil {
		s.l.Infow("cannot create google mail service", zap.Error(err))
		s.ms = nil
		return err
	} else {
		s.ms = srv
		return nil
	}
}

//Error Handling

//async
