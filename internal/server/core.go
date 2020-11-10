package server

import (
	"encoding/gob"
	"errors"
	"html/template"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/h44z/wg-portal/internal/wireguard"

	"github.com/h44z/wg-portal/internal/common"

	"github.com/h44z/wg-portal/internal/ldap"
	log "github.com/sirupsen/logrus"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
)

const SessionIdentifier = "wgPortalSession"
const CacheRefreshDuration = 5 * time.Minute

func init() {
	gob.Register(SessionData{})
	gob.Register(User{})
	gob.Register(Device{})
	gob.Register(LdapCreateForm{})
}

type SessionData struct {
	LoggedIn      bool
	IsAdmin       bool
	UID           string
	UserName      string
	Firstname     string
	Lastname      string
	Email         string
	SortedBy      string
	SortDirection string
	Search        string
	AlertData     string
	AlertType     string
	FormData      interface{}
}

type AlertData struct {
	HasAlert bool
	Message  string
	Type     string
}

type StaticData struct {
	WebsiteTitle string
	WebsiteLogo  string
	CompanyName  string
	Year         int
	LdapDisabled bool
}

type Server struct {
	// Core components
	config  *common.Config
	server  *gin.Engine
	users   *UserManager
	mailTpl *template.Template

	// WireGuard stuff
	wg *wireguard.Manager

	// LDAP stuff
	ldapDisabled     bool
	ldapAuth         ldap.Authentication
	ldapUsers        *ldap.SynchronizedUserCacheHolder
	ldapCacheUpdater *ldap.UserCache
}

func (s *Server) Setup() error {
	// Init rand
	rand.Seed(time.Now().UnixNano())

	s.config = common.NewConfig()

	// Setup LDAP stuff
	s.ldapAuth = ldap.NewAuthentication(s.config.LDAP)
	s.ldapUsers = &ldap.SynchronizedUserCacheHolder{}
	s.ldapUsers.Init()
	s.ldapCacheUpdater = ldap.NewUserCache(s.config.LDAP, s.ldapUsers)
	if s.ldapCacheUpdater.LastError != nil {
		log.Warnf("LDAP error: %v", s.ldapCacheUpdater.LastError)
		log.Warnf("LDAP features disabled!")
		s.ldapDisabled = true
	}

	// Setup WireGuard stuff
	s.wg = &wireguard.Manager{Cfg: &s.config.WG}
	if err := s.wg.Init(); err != nil {
		return err
	}

	// Setup user manager
	if s.users = NewUserManager(s.wg, s.ldapUsers); s.users == nil {
		return errors.New("unable to setup user manager")
	}
	if err := s.users.InitFromCurrentInterface(); err != nil {
		return errors.New("unable to initialize user manager")
	}
	if err := s.RestoreWireGuardInterface(); err != nil {
		return errors.New("unable to restore wirguard state")
	}

	dir := s.getExecutableDirectory()
	rDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	log.Infof("Real working directory: %s", rDir)
	log.Infof("Current working directory: %s", dir)
	var err error
	s.mailTpl, err = template.New("email.html").ParseFiles(filepath.Join(dir, "/assets/tpl/email.html"))
	if err != nil {
		return errors.New("unable to pare mail template")
	}

	// Setup http server
	s.server = gin.Default()

	// Setup templates
	log.Infof("Loading templates from: %s", filepath.Join(dir, "/assets/tpl/*.html"))
	s.server.LoadHTMLGlob(filepath.Join(dir, "/assets/tpl/*.html"))
	s.server.Use(sessions.Sessions("authsession", memstore.NewStore([]byte("secret")))) // TODO: change key?

	// Serve static files
	s.server.Static("/css", filepath.Join(dir, "/assets/css"))
	s.server.Static("/js", filepath.Join(dir, "/assets/js"))
	s.server.Static("/img", filepath.Join(dir, "/assets/img"))
	s.server.Static("/fonts", filepath.Join(dir, "/assets/fonts"))

	// Setup all routes
	SetupRoutes(s)

	log.Infof("Setup of service completed!")
	return nil
}

func (s *Server) Run() {
	// Start ldap group watcher
	if !s.ldapDisabled {
		go func(s *Server) {
			for {
				time.Sleep(CacheRefreshDuration)
				if err := s.ldapCacheUpdater.Update(true); err != nil {
					log.Warnf("Failed to update ldap group cache: %v", err)
				}
				log.Debugf("Refreshed LDAP permissions!")
			}
		}(s)
	}

	// Run web service
	err := s.server.Run(s.config.Core.ListeningAddress)
	if err != nil {
		log.Errorf("Failed to listen and serve on %s: %v", s.config.Core.ListeningAddress, err)
	}
}

func (s *Server) getExecutableDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Errorf("Failed to get executable directory: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "assets")); os.IsNotExist(err) {
		return "." // assets directory not found -> we are developing in goland =)
	}

	return dir
}

func (s *Server) getSessionData(c *gin.Context) SessionData {
	session := sessions.Default(c)
	rawSessionData := session.Get(SessionIdentifier)

	var sessionData SessionData
	if rawSessionData != nil {
		sessionData = rawSessionData.(SessionData)
	} else {
		sessionData = SessionData{
			SortedBy:      "mail",
			SortDirection: "asc",
			Email:         "",
			Firstname:     "",
			Lastname:      "",
			IsAdmin:       false,
			LoggedIn:      false,
		}
		session.Set(SessionIdentifier, sessionData)
		if err := session.Save(); err != nil {
			log.Errorf("Failed to store session: %v", err)
		}
	}

	return sessionData
}

func (s *Server) getAlertData(c *gin.Context) AlertData {
	currentSession := s.getSessionData(c)
	alertData := AlertData{
		HasAlert: currentSession.AlertData != "",
		Message:  currentSession.AlertData,
		Type:     currentSession.AlertType,
	}
	// Reset alerts
	_ = s.setAlert(c, "", "")

	return alertData
}

func (s *Server) updateSessionData(c *gin.Context, data SessionData) error {
	session := sessions.Default(c)
	session.Set(SessionIdentifier, data)
	if err := session.Save(); err != nil {
		log.Errorf("Failed to store session: %v", err)
		return err
	}
	return nil
}

func (s *Server) destroySessionData(c *gin.Context) error {
	session := sessions.Default(c)
	session.Delete(SessionIdentifier)
	if err := session.Save(); err != nil {
		log.Errorf("Failed to destroy session: %v", err)
		return err
	}
	return nil
}

func (s *Server) getStaticData() StaticData {
	return StaticData{
		WebsiteTitle: s.config.Core.Title,
		WebsiteLogo:  "/img/header-logo.png",
		CompanyName:  s.config.Core.CompanyName,
		LdapDisabled: s.ldapDisabled,
		Year:         time.Now().Year(),
	}
}

func (s *Server) setAlert(c *gin.Context, message, typ string) SessionData {
	currentSession := s.getSessionData(c)
	currentSession.AlertData = message
	currentSession.AlertType = typ
	_ = s.updateSessionData(c, currentSession)

	return currentSession
}

func (s SessionData) GetSortIcon(field string) string {
	if s.SortedBy != field {
		return "fa-sort"
	}
	if s.SortDirection == "asc" {
		return "fa-sort-alpha-down"
	} else {
		return "fa-sort-alpha-up"
	}
}
