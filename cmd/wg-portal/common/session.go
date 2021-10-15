package common

import (
	"encoding/gob"

	"github.com/h44z/wg-portal/internal/persistence"
)

func init() {
	gob.Register(SessionData{})
	gob.Register(FlashData{})
}

type SessionData struct {
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
