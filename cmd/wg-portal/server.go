package main

import (
	"context"
	"html/template"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/cmd/wg-portal/common"
	"github.com/h44z/wg-portal/cmd/wg-portal/restapi"
	"github.com/h44z/wg-portal/cmd/wg-portal/ui"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/h44z/wg-portal/internal/portal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	ginlogrus "github.com/toorop/gin-logrus"
)

type handler interface {
	RegisterRoutes(g *gin.Engine)
}

type server struct {
	config *common.Config

	server  *gin.Engine
	backend portal.Backend
}

func NewServer(config *common.Config) (*server, error) {
	s := &server{
		config: config,
	}

	// Database
	database, err := persistence.NewDatabase(config.Database)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to initialize persistent store")
	}

	// Portal Backend
	s.backend, err = portal.NewPersistentBackend(database)
	if err != nil {
		return nil, errors.WithMessagef(err, "backend failed to initialize")
	}

	// Web Handler
	err = s.setupGin()
	if err != nil {
		return nil, errors.WithMessagef(err, "backend failed to initialize")
	}

	// UI handler
	uiHandler, err := ui.NewHandler(s.config, s.backend)
	if err != nil {
		return nil, errors.WithMessagef(err, "ui handler failed to initialize")
	}
	uiHandler.RegisterRoutes(s.server)

	apiHandler, err := restapi.NewHandler(s.config, s.backend)
	if err != nil {
		return nil, errors.WithMessagef(err, "api handler failed to initialize")
	}
	apiHandler.RegisterRoutes(s.server)

	return s, nil
}

func (s *server) setupGin() error {
	// Web Handler
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = ioutil.Discard
	s.server = gin.New()
	if s.config.Core.GinDebug {
		gin.SetMode(gin.DebugMode)
		s.server.Use(ginlogrus.Logger(logrus.StandardLogger()))
	}
	s.server.Use(gin.Recovery())
	cookieStore := memstore.NewStore([]byte(s.config.Core.SessionSecret))
	cookieStore.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400, // auth session is valid for 1 day
		Secure:   strings.HasPrefix(s.config.Core.ExternalUrl, "https"),
		HttpOnly: true,
	})
	s.server.Use(sessions.Sessions("authsession", cookieStore))
	s.server.SetFuncMap(template.FuncMap{
		"urlEncode":  url.QueryEscape,
		"startsWith": strings.HasPrefix,
	})

	// Setup templates
	templates := template.Must(template.New("").Funcs(s.server.FuncMap).ParseFS(Templates, "assets/tpl/*.html"))
	s.server.SetHTMLTemplate(templates)

	// Serve static files
	s.server.StaticFS("/css", http.FS(fsMust(fs.Sub(Statics, "assets/css"))))
	s.server.StaticFS("/js", http.FS(fsMust(fs.Sub(Statics, "assets/js"))))
	s.server.StaticFS("/img", http.FS(fsMust(fs.Sub(Statics, "assets/img"))))
	s.server.StaticFS("/fonts", http.FS(fsMust(fs.Sub(Statics, "assets/fonts"))))

	return nil
}

func (s *server) Run(ctx context.Context) {
	logrus.Infof("starting web server on %s", s.config.Core.ListeningAddress)

	// Run web service
	srv := &http.Server{
		Addr:    s.config.Core.ListeningAddress,
		Handler: s.server,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logrus.Tracef("web service on %s exited: %v", s.config.Core.ListeningAddress, err)
		}
	}()

	// Wait for the main context to end
	<-ctx.Done()

	logrus.Debug("web server shutting down, grace period: 5 seconds...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	logrus.Info("web server shut down")
}

func (s *server) Shutdown() {
	// TODO: run cleanup stuff
}

func fsMust(f fs.FS, err error) fs.FS {
	if err != nil {
		panic(err)
	}
	return f
}
