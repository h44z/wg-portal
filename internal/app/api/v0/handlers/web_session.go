package handlers

import (
	"context"
	"encoding/gob"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/h44z/wg-portal/internal/config"
)

func init() {
	gob.Register(SessionData{})
}

type SessionData struct {
	LoggedIn bool
	IsAdmin  bool

	UserIdentifier string

	Firstname string
	Lastname  string
	Email     string

	OauthState    string
	OauthNonce    string
	OauthProvider string
	OauthReturnTo string

	WebAuthnData string

	CsrfToken string
}

const sessionApiV0Key = "session_api_v0"

type SessionWrapper struct {
	*scs.SessionManager
}

func NewSessionWrapper(cfg *config.Config) *SessionWrapper {
	sessionManager := scs.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Name = cfg.Web.SessionIdentifier
	sessionManager.Cookie.Secure = strings.HasPrefix(cfg.Web.ExternalUrl, "https")
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Path = "/"
	sessionManager.Cookie.Persist = false

	wrappedSessionManager := &SessionWrapper{sessionManager}

	return wrappedSessionManager
}

func (s *SessionWrapper) SetData(ctx context.Context, value SessionData) {
	s.SessionManager.Put(ctx, sessionApiV0Key, value)
}

func (s *SessionWrapper) GetData(ctx context.Context) SessionData {
	sessionData, ok := s.SessionManager.Get(ctx, sessionApiV0Key).(SessionData)
	if !ok {
		return s.defaultSessionData()
	}
	return sessionData
}

func (s *SessionWrapper) DestroyData(ctx context.Context) {
	_ = s.SessionManager.Destroy(ctx)
}

func (s *SessionWrapper) defaultSessionData() SessionData {
	return SessionData{
		LoggedIn:       false,
		IsAdmin:        false,
		UserIdentifier: "",
		Firstname:      "",
		Lastname:       "",
		Email:          "",
		OauthState:     "",
		OauthNonce:     "",
		OauthProvider:  "",
		OauthReturnTo:  "",
	}
}
