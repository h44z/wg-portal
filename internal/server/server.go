package server

import (
	"context"
	"encoding/gob"
	"html/template"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	wgportal "github.com/h44z/wg-portal"
	ldapprovider "github.com/h44z/wg-portal/internal/authentication/providers/ldap"
	passwordprovider "github.com/h44z/wg-portal/internal/authentication/providers/password"
	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/h44z/wg-portal/internal/wireguard"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	ginlogrus "github.com/toorop/gin-logrus"
	csrf "github.com/utrack/gin-csrf"
	"gorm.io/gorm"
)

const SessionIdentifier = "wgPortalSession"

func init() {
	gob.Register(SessionData{})
	gob.Register(FlashData{})
	gob.Register(wireguard.Peer{})
	gob.Register(wireguard.Device{})
	gob.Register(LdapCreateForm{})
	gob.Register(users.User{})
}

type SessionData struct {
	LoggedIn   bool
	IsAdmin    bool
	Firstname  string
	Lastname   string
	Email      string
	DeviceName string

	SortedBy      map[string]string
	SortDirection map[string]string
	Search        map[string]string

	AlertData string
	AlertType string
	FormData  interface{}
}

type FlashData struct {
	HasAlert bool
	Message  string
	Type     string
}

type StaticData struct {
	WebsiteTitle string
	WebsiteLogo  string
	CompanyName  string
	Year         int
}

type Server struct {
	ctx     context.Context
	config  *Config
	server  *gin.Engine
	mailTpl *template.Template
	auth    *AuthManager

	db    *gorm.DB
	users *users.Manager
	wg    *wireguard.Manager
	peers *wireguard.PeerManager
}

func (s *Server) Setup(ctx context.Context) error {
	var err error

	dir := s.getExecutableDirectory()
	rDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	logrus.Infof("real working directory: %s", rDir)
	logrus.Infof("current working directory: %s", dir)

	// Init rand
	rand.Seed(time.Now().UnixNano())

	s.config = NewConfig()
	s.ctx = ctx

	// Setup database connection
	s.db, err = common.GetDatabaseForConfig(&s.config.Database)
	if err != nil {
		return errors.WithMessage(err, "database setup failed")
	}
	err = common.MigrateDatabase(s.db, Version)
	if err != nil {
		return errors.WithMessage(err, "database migration failed")
	}

	// Setup http server
	gin.SetMode(gin.DebugMode)
	gin.DefaultWriter = ioutil.Discard
	s.server = gin.New()
	if logrus.GetLevel() == logrus.TraceLevel {
		s.server.Use(ginlogrus.Logger(logrus.StandardLogger()))
	}
	s.server.Use(gin.Recovery())
	s.server.Use(sessions.Sessions("authsession", memstore.NewStore([]byte(s.config.Core.SessionSecret))))
	s.server.Use(csrf.Middleware(csrf.Options{
		Secret: s.config.Core.SessionSecret,
		ErrorFunc: func(c *gin.Context) {
			c.String(400, "CSRF token mismatch")
			c.Abort()
		},
	}))
	s.server.SetFuncMap(template.FuncMap{
		"formatBytes": common.ByteCountSI,
		"urlEncode":   url.QueryEscape,
		"startsWith":  strings.HasPrefix,
		"userForEmail": func(users []users.User, email string) *users.User {
			for i := range users {
				if users[i].Email == email {
					return &users[i]
				}
			}
			return nil
		},
	})

	// Setup templates
	templates := template.Must(template.New("").Funcs(s.server.FuncMap).ParseFS(wgportal.Templates, "assets/tpl/*.html"))
	s.server.SetHTMLTemplate(templates)

	// Serve static files
	s.server.StaticFS("/css", http.FS(fsMust(fs.Sub(wgportal.Statics, "assets/css"))))
	s.server.StaticFS("/js", http.FS(fsMust(fs.Sub(wgportal.Statics, "assets/js"))))
	s.server.StaticFS("/img", http.FS(fsMust(fs.Sub(wgportal.Statics, "assets/img"))))
	s.server.StaticFS("/fonts", http.FS(fsMust(fs.Sub(wgportal.Statics, "assets/fonts"))))

	// Setup all routes
	SetupRoutes(s)

	// Setup user database (also needed for database authentication)
	s.users, err = users.NewManager(s.db)
	if err != nil {
		return errors.WithMessage(err, "user-manager initialization failed")
	}

	// Setup auth manager
	s.auth = NewAuthManager(s)
	pwProvider, err := passwordprovider.New(&s.config.Database)
	if err != nil {
		return errors.WithMessage(err, "password provider initialization failed")
	}
	if err = pwProvider.InitializeAdmin(s.config.Core.AdminUser, s.config.Core.AdminPassword); err != nil {
		return errors.WithMessage(err, "admin initialization failed")
	}
	s.auth.RegisterProvider(pwProvider)

	if s.config.Core.LdapEnabled {
		ldapProvider, err := ldapprovider.New(&s.config.LDAP)
		if err != nil {
			s.config.Core.LdapEnabled = false
			logrus.Warnf("failed to setup LDAP connection, LDAP features disabled")
		}
		s.auth.RegisterProviderWithoutError(ldapProvider, err)
	}

	// Setup WireGuard stuff
	s.wg = &wireguard.Manager{Cfg: &s.config.WG}
	if err = s.wg.Init(); err != nil {
		return errors.WithMessage(err, "unable to initialize WireGuard manager")
	}

	// Setup peer manager
	if s.peers, err = wireguard.NewPeerManager(s.db, s.wg); err != nil {
		return errors.WithMessage(err, "unable to setup peer manager")
	}

	for _, deviceName := range s.wg.Cfg.DeviceNames {
		if err = s.RestoreWireGuardInterface(deviceName); err != nil {
			return errors.WithMessagef(err, "unable to restore WireGuard state for %s", deviceName)
		}
	}

	// Setup mail template
	s.mailTpl, err = template.New("email.html").ParseFS(wgportal.Templates, "assets/tpl/email.html")
	if err != nil {
		return errors.Wrap(err, "unable to pare mail template")
	}

	logrus.Infof("setup of service completed!")
	return nil
}

func (s *Server) Run() {
	logrus.Infof("starting web service on %s", s.config.Core.ListeningAddress)

	// Start ldap sync
	if s.config.Core.LdapEnabled {
		go s.SyncLdapWithUserDatabase()
	}

	// Run web service
	srv := &http.Server{
		Addr:    s.config.Core.ListeningAddress,
		Handler: s.server,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logrus.Debugf("web service on %s exited: %v", s.config.Core.ListeningAddress, err)
		}
	}()

	<-s.ctx.Done()

	logrus.Debug("web service shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

}

func (s *Server) getExecutableDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logrus.Errorf("failed to get executable directory: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "assets")); os.IsNotExist(err) {
		return "." // assets directory not found -> we are developing in goland =)
	}

	return dir
}

func (s *Server) getStaticData() StaticData {
	return StaticData{
		WebsiteTitle: s.config.Core.Title,
		WebsiteLogo:  "/img/header-logo.png",
		CompanyName:  s.config.Core.CompanyName,
		Year:         time.Now().Year(),
	}
}

func GetSessionData(c *gin.Context) SessionData {
	session := sessions.Default(c)
	rawSessionData := session.Get(SessionIdentifier)

	var sessionData SessionData
	if rawSessionData != nil {
		sessionData = rawSessionData.(SessionData)
	} else {
		sessionData = SessionData{
			Search:        map[string]string{"peers": "", "userpeers": "", "users": ""},
			SortedBy:      map[string]string{"peers": "handshake", "userpeers": "id", "users": "email"},
			SortDirection: map[string]string{"peers": "desc", "userpeers": "asc", "users": "asc"},
			Email:         "",
			Firstname:     "",
			Lastname:      "",
			DeviceName:    "",
			IsAdmin:       false,
			LoggedIn:      false,
		}
		session.Set(SessionIdentifier, sessionData)
		if err := session.Save(); err != nil {
			logrus.Errorf("failed to store session: %v", err)
		}
	}

	return sessionData
}

func GetFlashes(c *gin.Context) []FlashData {
	session := sessions.Default(c)
	flashes := session.Flashes()
	if err := session.Save(); err != nil {
		logrus.Errorf("failed to store session after setting flash: %v", err)
	}

	flashData := make([]FlashData, len(flashes))
	for i := range flashes {
		flashData[i] = flashes[i].(FlashData)
	}

	return flashData
}

func UpdateSessionData(c *gin.Context, data SessionData) error {
	session := sessions.Default(c)
	session.Set(SessionIdentifier, data)
	if err := session.Save(); err != nil {
		logrus.Errorf("failed to store session: %v", err)
		return errors.Wrap(err, "failed to store session")
	}
	return nil
}

func DestroySessionData(c *gin.Context) error {
	session := sessions.Default(c)
	session.Delete(SessionIdentifier)
	if err := session.Save(); err != nil {
		logrus.Errorf("failed to destroy session: %v", err)
		return errors.Wrap(err, "failed to destroy session")
	}
	return nil
}

func SetFlashMessage(c *gin.Context, message, typ string) {
	session := sessions.Default(c)
	session.AddFlash(FlashData{
		Message: message,
		Type:    typ,
	})
	if err := session.Save(); err != nil {
		logrus.Errorf("failed to store session after setting flash: %v", err)
	}
}

func (s SessionData) GetSortIcon(table, field string) string {
	if s.SortedBy[table] != field {
		return "fa-sort"
	}
	if s.SortDirection[table] == "asc" {
		return "fa-sort-alpha-down"
	} else {
		return "fa-sort-alpha-up"
	}
}

func fsMust(f fs.FS, err error) fs.FS {
	if err != nil {
		panic(err)
	}
	return f
}
