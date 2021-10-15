package ui

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal"
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
