package ui

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/cmd/wg-portal/common"
	"github.com/h44z/wg-portal/internal/portal"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	config *common.Config

	backend portal.Backend
}

func NewHandler(config *common.Config, backend portal.Backend) (*Handler, error) {
	h := &Handler{
		config:  config,
		backend: backend,
	}
	return h, nil
}

func (h *Handler) RegisterRoutes(g *gin.Engine) {
	g.GET("/", h.GetIndex)
}

//
// --
//

const SessionIdentifier = "wgPortalSession"

type StaticData struct {
	WebsiteTitle string
	WebsiteLogo  string
	CompanyName  string
	Year         int
	Version      string
}

func GetSessionData(c *gin.Context) common.SessionData {
	session := sessions.Default(c)
	rawSessionData := session.Get(SessionIdentifier)

	var sessionData common.SessionData
	if rawSessionData != nil {
		sessionData = rawSessionData.(common.SessionData)
	} else {
		sessionData = common.SessionData{
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
		session.Set(SessionIdentifier, sessionData)
		if err := session.Save(); err != nil {
			logrus.Errorf("failed to store session: %v", err)
		}
	}

	return sessionData
}

func GetFlashes(c *gin.Context) []common.FlashData {
	session := sessions.Default(c)
	flashes := session.Flashes()
	if err := session.Save(); err != nil {
		logrus.Errorf("failed to store session after setting flash: %v", err)
	}

	flashData := make([]common.FlashData, len(flashes))
	for i := range flashes {
		flashData[i] = flashes[i].(common.FlashData)
	}

	return flashData
}
