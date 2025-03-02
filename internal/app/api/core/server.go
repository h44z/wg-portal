package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
)

var (
	random = rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
)

const (
	RequestIDKey = "X-Request-ID"
)

type ApiVersion string
type HandlerName string

type GroupSetupFn func(group *gin.RouterGroup)

type ApiEndpointSetupFunc func() (ApiVersion, GroupSetupFn)

type Server struct {
	cfg      *config.Config
	server   *gin.Engine
	versions map[ApiVersion]*gin.RouterGroup
}

func NewServer(cfg *config.Config, endpoints ...ApiEndpointSetupFunc) (*Server, error) {
	s := &Server{
		cfg: cfg,
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "apiserver"
	}
	hostname += ", version " + internal.Version

	// Setup http server
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	s.server = gin.New()

	if cfg.Web.RequestLogging {
		if cfg.Advanced.LogLevel == "trace" {
			gin.SetMode(gin.DebugMode)
		}
		s.server.Use(func(c *gin.Context) {
			start := time.Now()
			path := c.Request.URL.Path
			raw := c.Request.URL.RawQuery

			c.Next()

			if raw != "" {
				path = path + "?" + raw
			}

			latency := time.Since(start)
			status := c.Writer.Status()
			clientIP := c.ClientIP()
			method := c.Request.Method
			errorMsg := c.Errors.ByType(gin.ErrorTypePrivate).String()

			slog.Debug("HTTP Request",
				"status", status,
				"latency", latency,
				"client", clientIP,
				"method", method,
				"path", path,
				"error", errorMsg,
			)
		})
	}

	s.server.Use(gin.Recovery()).Use(func(c *gin.Context) {
		c.Writer.Header().Set("X-Served-By", hostname)
		c.Next()
	}).Use(func(c *gin.Context) {
		xRequestID := uuid(16)

		c.Request.Header.Set(RequestIDKey, xRequestID)
		c.Set(RequestIDKey, xRequestID)
		c.Next()
	})

	// Setup templates
	templates := template.Must(template.New("").Funcs(s.server.FuncMap).ParseFS(apiTemplates, "assets/tpl/*.gohtml"))
	s.server.SetHTMLTemplate(templates)

	// Serve static files
	imgFs := http.FS(fsMust(fs.Sub(apiStatics, "assets/img")))
	s.server.StaticFS("/css", http.FS(fsMust(fs.Sub(apiStatics, "assets/css"))))
	s.server.StaticFS("/js", http.FS(fsMust(fs.Sub(apiStatics, "assets/js"))))
	s.server.StaticFS("/img", imgFs)
	s.server.StaticFS("/fonts", http.FS(fsMust(fs.Sub(apiStatics, "assets/fonts"))))
	s.server.StaticFS("/doc", http.FS(fsMust(fs.Sub(apiStatics, "assets/doc"))))

	// Setup routes
	s.server.UseRawPath = true
	s.server.UnescapePathValues = true
	s.setupRoutes(endpoints...)
	s.setupFrontendRoutes()

	return s, nil
}

func (s *Server) Run(ctx context.Context, listenAddress string) {
	// Run web service
	srv := &http.Server{
		Addr:    listenAddress,
		Handler: s.server,
	}

	srvContext, cancelFn := context.WithCancel(ctx)
	go func() {
		var err error
		if s.cfg.Web.CertFile != "" && s.cfg.Web.KeyFile != "" {
			err = srv.ListenAndServeTLS(s.cfg.Web.CertFile, s.cfg.Web.KeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil {
			slog.Info("web service exited",
				"address", listenAddress,
				"error", err)
			cancelFn()
		}
	}()
	slog.Info("started web service", "address", listenAddress)

	// Wait for the main context to end
	<-srvContext.Done()

	slog.Debug("web service shutting down, grace period: 5 seconds")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	slog.Debug("web service shut down")
}

func (s *Server) setupRoutes(endpoints ...ApiEndpointSetupFunc) {
	s.server.GET("/api", s.landingPage)
	s.versions = make(map[ApiVersion]*gin.RouterGroup)

	for _, setupFunc := range endpoints {
		version, groupSetupFn := setupFunc()

		if _, ok := s.versions[version]; !ok {
			s.versions[version] = s.server.Group(fmt.Sprintf("/api/%s", version))

			// OpenAPI documentation (via RapiDoc)
			s.versions[version].GET("/swagger/index.html", s.rapiDocHandler(version)) // Deprecated: old link
			s.versions[version].GET("/doc.html", s.rapiDocHandler(version))

			groupSetupFn(s.versions[version])
		}
	}
}

func (s *Server) setupFrontendRoutes() {
	// Serve static files
	s.server.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/app")
	})
	s.server.GET("/favicon.ico", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/app/favicon.ico")
	})
	s.server.StaticFS("/app", http.FS(fsMust(fs.Sub(frontendStatics, "frontend-dist"))))
}

func (s *Server) landingPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.gohtml", gin.H{
		"Version": internal.Version,
		"Year":    time.Now().Year(),
	})
}

func (s *Server) rapiDocHandler(version ApiVersion) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "rapidoc.gohtml", gin.H{
			"RapiDocSource": "/js/rapidoc-min.js",
			"ApiSpecUrl":    fmt.Sprintf("/doc/%s_swagger.yaml", version),
			"Version":       internal.Version,
			"Year":          time.Now().Year(),
		})
	}
}

func fsMust(f fs.FS, err error) fs.FS {
	if err != nil {
		panic(err)
	}
	return f
}

func uuid(len int) string {
	bytes := make([]byte, len)
	random.Read(bytes)
	return base64.StdEncoding.EncodeToString(bytes)[:len]
}
