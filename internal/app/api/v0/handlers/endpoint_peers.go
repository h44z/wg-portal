package handlers

import (
	model2 "github.com/h44z/wg-portal/internal/app/api/v0/model"
	"net/http"

	"github.com/h44z/wg-portal/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
)

type peerEndpoint struct {
	app           *app.App
	authenticator *authenticationHandler
}

func (e peerEndpoint) GetName() string {
	return "PeerEndpoint"
}

func (e peerEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/peer", e.authenticator.LoggedIn())

	apiGroup.GET("/all/:id", e.handlePeersGet())
}

// handlePeersGet returns a gorm handler function.
//
// @ID peers_handlePeersGet
// @Tags Peer
// @Summary Get peers for the given interface.
// @Produce json
// @Success 200 {object} []model.Peer
// @Failure 500 {object} model.Error
// @Router /peer/all/{id} [get]
func (e peerEndpoint) handlePeersGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		interfaceId := c.Param("id")
		if interfaceId == "" {
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		_, peers, err := e.app.WireGuard.GetInterfaceAndPeers(c.Request.Context(), domain.InterfaceIdentifier(interfaceId))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model2.NewPeers(peers))
	}
}