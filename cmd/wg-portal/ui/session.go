package ui

import (
	"encoding/gob"
	"fmt"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/persistence"
)

func init() {
	gob.Register(SessionData{})
	gob.Register(FlashData{})
}

type SessionData struct {
	AuthBackend         string
	OauthState          string // oauth state
	OidcNonce           string // oidc id token nonce
	LoggedIn            bool
	IsAdmin             bool
	UserIdentifier      persistence.UserIdentifier
	Firstname           string
	Lastname            string
	Email               string
	InterfaceIdentifier persistence.InterfaceIdentifier

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

type SessionStore interface {
	GetData(c *gin.Context) SessionData
	SetData(c *gin.Context, data SessionData)

	GetFlashes(c *gin.Context) []FlashData
	SetFlashes(c *gin.Context, flashes ...FlashData)

	RemoveData(c *gin.Context)
	RemoveFlashes(c *gin.Context)
}

type GinSessionStore struct {
	sessionIdentifier string
}

func (g GinSessionStore) GetData(c *gin.Context) SessionData {
	session := sessions.Default(c)
	rawSessionData := session.Get(g.sessionIdentifier)

	var sessionData SessionData
	if rawSessionData != nil {
		sessionData = rawSessionData.(SessionData)
	} else {
		// init a new default session
		sessionData = SessionData{
			Search:              map[string]string{"peers": "", "userpeers": "", "users": ""},
			SortedBy:            map[string]string{"peers": "handshake", "userpeers": "id", "users": "email"},
			SortDirection:       map[string]string{"peers": "desc", "userpeers": "asc", "users": "asc"},
			Email:               "",
			Firstname:           "",
			Lastname:            "",
			InterfaceIdentifier: "",
			IsAdmin:             false,
			LoggedIn:            false,
		}
		session.Set(g.sessionIdentifier, sessionData)
		if err := session.Save(); err != nil {
			panic(fmt.Sprintf("failed to store session: %v", err))
		}
	}

	return sessionData
}

func (g GinSessionStore) SetData(c *gin.Context, data SessionData) {
	session := sessions.Default(c)
	session.Set(g.sessionIdentifier, data)
	if err := session.Save(); err != nil {
		panic(fmt.Sprintf("failed to store session: %v", err))
	}
}

func (g GinSessionStore) GetFlashes(c *gin.Context) []FlashData {
	session := sessions.Default(c)
	flashes := session.Flashes()
	if err := session.Save(); err != nil {
		panic(fmt.Sprintf("failed to store session: %v", err))
	}

	flashData := make([]FlashData, len(flashes))
	for i := range flashes {
		flashData[i] = flashes[i].(FlashData)
	}

	return flashData
}

func (g GinSessionStore) SetFlashes(c *gin.Context, flashes ...FlashData) {
	session := sessions.Default(c)
	for i := range flashes {
		session.AddFlash(flashes[i])
	}
	if err := session.Save(); err != nil {
		panic(fmt.Sprintf("failed to store session: %v", err))
	}
}

func (g GinSessionStore) RemoveData(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(g.sessionIdentifier)
	if err := session.Save(); err != nil {
		panic(fmt.Sprintf("failed to store session: %v", err))
	}
}

func (g GinSessionStore) RemoveFlashes(c *gin.Context) {
	session := sessions.Default(c)
	_ = session.Flashes() // Clear flashes
	if err := session.Save(); err != nil {
		panic(fmt.Sprintf("failed to store session: %v", err))
	}
}
