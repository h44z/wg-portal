package core

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/app/api/core/middleware/cors"
	"github.com/h44z/wg-portal/internal/app/api/core/middleware/logging"
	"github.com/h44z/wg-portal/internal/app/api/core/middleware/recovery"
	"github.com/h44z/wg-portal/internal/app/api/core/middleware/tracing"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/config"
)

const (
	RequestIDKey = "X-Request-ID"
)

type ApiVersion string
type HandlerName string

type GroupSetupFn func(group *routegroup.Bundle)

type ApiEndpointSetupFunc func() (ApiVersion, GroupSetupFn)

type Server struct {
	cfg      *config.Config
	server   *routegroup.Bundle
	tpl      *respond.TemplateRenderer
	versions map[ApiVersion]*routegroup.Bundle
}

func NewServer(cfg *config.Config, endpoints ...ApiEndpointSetupFunc) (*Server, error) {
	s := &Server{
		cfg:    cfg,
		server: routegroup.New(http.NewServeMux()),
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "apiserver"
	}
	hostname += ", version " + internal.Version

	s.server.Use(recovery.New().Handler)
	if cfg.Web.RequestLogging {
		s.server.Use(logging.New(logging.WithLevel(logging.LogLevelDebug)).Handler)

	}
	s.server.Use(cors.New().Handler)
	s.server.Use(tracing.New(
		tracing.WithContextIdentifier(RequestIDKey),
		tracing.WithHeaderIdentifier(RequestIDKey),
	).Handler)
	if cfg.Web.ExposeHostInfo {
		s.server.Use(func(handler http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Served-By", hostname)
				handler.ServeHTTP(w, r)
			})
		})
	}

	// Setup templates
	s.tpl = respond.NewTemplateRenderer(
		template.Must(template.New("").ParseFS(apiTemplates, "assets/tpl/*.gohtml")),
	)

	// Serve static files
	imgFs := http.FS(fsMust(fs.Sub(apiStatics, "assets/img")))
	s.server.HandleFiles("/css", http.FS(fsMust(fs.Sub(apiStatics, "assets/css"))))
	s.server.HandleFiles("/js", http.FS(fsMust(fs.Sub(apiStatics, "assets/js"))))
	s.server.HandleFiles("/img", imgFs)
	s.server.HandleFiles("/fonts", http.FS(fsMust(fs.Sub(apiStatics, "assets/fonts"))))
	s.server.HandleFiles("/doc", http.FS(fsMust(fs.Sub(apiStatics, "assets/doc"))))

	// Setup routes
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
		slog.Debug("starting server", "certFile", s.cfg.Web.CertFile, "keyFile", s.cfg.Web.KeyFile)
		if s.cfg.Web.CertFile != "" && s.cfg.Web.KeyFile != "" {
			err = srv.ListenAndServeTLS(s.cfg.Web.CertFile, s.cfg.Web.KeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil {
			slog.Info("web service exited", "address", listenAddress, "error", err)
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
	s.server.HandleFunc("GET /api", s.landingPage)
	s.versions = make(map[ApiVersion]*routegroup.Bundle)

	for _, setupFunc := range endpoints {
		version, groupSetupFn := setupFunc()

		if _, ok := s.versions[version]; !ok {
			s.versions[version] = s.server.Mount(fmt.Sprintf("/api/%s", version))

			// OpenAPI documentation (via RapiDoc)
			s.versions[version].HandleFunc("GET /swagger/index.html", s.rapiDocHandler(version)) // Deprecated: old link
			s.versions[version].HandleFunc("GET /doc.html", s.rapiDocHandler(version))

			versionGroup := s.versions[version].Group()
			groupSetupFn(versionGroup)
		}
	}
}

func (s *Server) setupFrontendRoutes() {
	// Serve static files
	s.server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		respond.Redirect(w, r, http.StatusMovedPermanently, "/app")
	})

	s.server.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		respond.Redirect(w, r, http.StatusMovedPermanently, "/app/favicon.ico")
	})

	// If a custom frontend path is configured, serve files from there when it contains content.
	// If the directory is empty or missing, populate it with the embedded frontend-dist content first.
	if s.cfg.Web.FrontendFilePath != "" {
		if err := os.MkdirAll(s.cfg.Web.FrontendFilePath, 0755); err != nil {
			slog.Error("failed to create frontend base directory", "path", s.cfg.Web.FrontendFilePath, "error", err)
		} else {
			ok := true
			hasFiles, err := dirHasFiles(s.cfg.Web.FrontendFilePath)
			if err != nil {
				slog.Error("failed to check frontend base directory", "path", s.cfg.Web.FrontendFilePath, "error", err)
				ok = false
			}
			if !hasFiles && ok {
				embeddedFS := fsMust(fs.Sub(frontendStatics, "frontend-dist"))
				if err := copyEmbedDirToDisk(embeddedFS, s.cfg.Web.FrontendFilePath); err != nil {
					slog.Error("failed to populate frontend base directory from embedded assets",
						"path", s.cfg.Web.FrontendFilePath, "error", err)
					ok = false
				}
			}

			if ok {
				// serve files from FS
				slog.Debug("serving frontend files from custom path", "path", s.cfg.Web.FrontendFilePath)
				s.server.HandleFiles("/app", http.Dir(s.cfg.Web.FrontendFilePath))
				return
			}
		}
	}

	// Fallback: serve embedded frontend files
	s.server.HandleFiles("/app", http.FS(fsMust(fs.Sub(frontendStatics, "frontend-dist"))))
}

func (s *Server) landingPage(w http.ResponseWriter, _ *http.Request) {
	s.tpl.HTML(w, http.StatusOK, "index.gohtml", respond.TplData{
		"Version": internal.Version,
		"Year":    time.Now().Year(),
	})
}

func (s *Server) rapiDocHandler(version ApiVersion) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.tpl.HTML(w, http.StatusOK, "rapidoc.gohtml", respond.TplData{
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

// dirHasFiles returns true if the directory contains at least one file (non-directory).
func dirHasFiles(dir string) (bool, error) {
	d, err := os.Open(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer d.Close()

	// Read a few entries; if any entry exists, consider it having files/dirs.
	// We want to know if there is at least one file; if only subdirs exist, still treat as content.
	entries, err := d.Readdir(-1)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.IsDir() {
			// check recursively
			has, err := dirHasFiles(filepath.Join(dir, e.Name()))
			if err == nil && has {
				return true, nil
			}
			continue
		}
		// regular file
		return true, nil
	}
	return false, nil
}

// copyEmbedDirToDisk copies the contents of srcFS into dstDir on disk.
func copyEmbedDirToDisk(srcFS fs.FS, dstDir string) error {
	return fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		// ensure parent dir exists
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		// open source file
		f, err := srcFS.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, f); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}
