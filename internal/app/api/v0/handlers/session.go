package handlers

import (
	"encoding/gob"
	"fmt"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
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
}

type SessionStore interface {
	DefaultSessionData() SessionData

	GetData(c *gin.Context) SessionData
	SetData(c *gin.Context, data SessionData)

	DestroyData(c *gin.Context)
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

func (g GinSessionStore) SetData(c *gin.Context, data SessionData) {
	session := sessions.Default(c)
	session.Set(g.sessionIdentifier, data)
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
