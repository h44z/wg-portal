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
	gob.Register(ErrorData{})
}

type SessionData struct {
	DeepLink string // deep link, used to redirect after a successful login

	OauthState string // oauth state
	OidcNonce  string // oidc id token nonce

	LoggedIn       bool
	IsAdmin        bool
	UserIdentifier persistence.UserIdentifier
	Firstname      string
	Lastname       string
	Email          string

	// currently selected interface
	InterfaceIdentifier persistence.InterfaceIdentifier

	// table sorting and paging
	SortedBy      map[string]string
	SortDirection map[string]string
	Search        map[string]string
	CurrentPage   int
	PageSize      int

	// alert that is printed on top of the page
	AlertData string
	AlertType string

	// currently filled form data
	FormData interface{}

	// Session error
	Error *ErrorData
}

type ErrorData struct {
	Message string
	Details string
	Code    int
	Path    string
}

type FlashData struct {
	Message string
	Type    string // flash type, for example: danger, success, warning, info, primary
}

type SessionStore interface {
	DefaultSessionData() SessionData

	GetData(c *gin.Context) SessionData
	SetData(c *gin.Context, data SessionData)

	GetFlashes(c *gin.Context) []FlashData
	SetFlashes(c *gin.Context, flashes ...FlashData)

	DestroyData(c *gin.Context)
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
		sessionData = g.DefaultSessionData()
		session.Set(g.sessionIdentifier, sessionData)
		if err := session.Save(); err != nil {
			panic(fmt.Sprintf("failed to store session: %v", err))
		}
	}

	return sessionData
}

func (g GinSessionStore) DefaultSessionData() SessionData {
	return SessionData{
		Search:              map[string]string{"peers": "", "userpeers": "", "users": ""},
		SortedBy:            map[string]string{"peers": "id", "userpeers": "id", "users": "email"},
		SortDirection:       map[string]string{"peers": "asc", "userpeers": "asc", "users": "asc"},
		CurrentPage:         1,
		PageSize:            25,
		Email:               "",
		Firstname:           "",
		Lastname:            "",
		InterfaceIdentifier: "",
		IsAdmin:             false,
		LoggedIn:            false,
	}
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

func (g GinSessionStore) DestroyData(c *gin.Context) {
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
