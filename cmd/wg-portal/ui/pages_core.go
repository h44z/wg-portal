package ui

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal"
	csrf "github.com/utrack/gin-csrf"
)

func (h *Handler) getStaticData() StaticData {
	return StaticData{
		WebsiteTitle: h.config.Core.Title,
		WebsiteLogo:  h.config.Core.LogoUrl,
		CompanyName:  h.config.Core.CompanyName,
		Year:         time.Now().Year(),
		Version:      internal.Version,
	}
}

func (h *Handler) GetIndex(c *gin.Context) {
	currentSession := GetSessionData(c)

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Route":          c.Request.URL.Path,
		"Alerts":         GetFlashes(c),
		"Session":        currentSession,
		"Static":         h.getStaticData(),
		"Interface":      nil, // TODO: load interface specified in the session
		"InterfaceNames": map[string]string{"wgX": "wgX descr"},
	})
}

func (h *Handler) GetLogin(c *gin.Context) {
	currentSession := GetSessionData(c)
	if currentSession.LoggedIn {
		c.Redirect(http.StatusSeeOther, "/") // already logged in
	}

	deepLink := c.DefaultQuery("dl", "")
	authError := c.DefaultQuery("err", "")
	errMsg := "Unknown error occurred, try again!"
	switch authError {
	case "missingdata":
		errMsg = "Invalid login data retrieved, please fill out all fields and try again!"
	case "authfail":
		errMsg = "Authentication failed!"
	case "loginreq":
		errMsg = "Login required!"
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"HasError": authError != "",
		"Message":  errMsg,
		"DeepLink": deepLink,
		"Static":   h.getStaticData(),
		"Csrf":     csrf.GetToken(c),
	})
}
