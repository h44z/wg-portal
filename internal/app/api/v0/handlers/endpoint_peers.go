package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
	"net/http"
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

	apiGroup.GET("/iface/:iface/all", e.handleAllGet())
	apiGroup.GET("/iface/:iface/prepare", e.handlePrepareGet())
	apiGroup.POST("/iface/:iface/new", e.handleCreatePost())
	apiGroup.GET("/:id", e.handleSingleGet())
	apiGroup.PUT("/:id", e.handleUpdatePut())
	apiGroup.DELETE("/:id", e.handleDelete())
}

// handleAllGet returns a gorm handler function.
//
// @ID peers_handleAllGet
// @Tags Peer
// @Summary Get peers for the given interface.
// @Produce json
// @Param iface path string true "The interface identifier"
// @Success 200 {object} []model.Peer
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/iface/{iface}/all [get]
func (e peerEndpoint) handleAllGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		interfaceId := c.Param("iface")
		if interfaceId == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing iface parameter"})
			return
		}

		_, peers, err := e.app.GetInterfaceAndPeers(ctx, domain.InterfaceIdentifier(interfaceId))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewPeers(peers))
	}
}

// handleSingleGet returns a gorm handler function.
//
// @ID peers_handleSingleGet
// @Tags Peer
// @Summary Get peer for the given identifier.
// @Produce json
// @Param id path string true "The peer identifier"
// @Success 200 {object} model.Peer
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/{id} [get]
func (e peerEndpoint) handleSingleGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		peerId := c.Param("id")
		if peerId == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing id parameter"})
			return
		}

		peer, err := e.app.GetPeer(ctx, domain.PeerIdentifier(peerId))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewPeer(peer))
	}
}

// handlePrepareGet returns a gorm handler function.
//
// @ID peers_handlePrepareGet
// @Tags Peer
// @Summary Prepare a new peer for the given interface.
// @Produce json
// @Param iface path string true "The interface identifier"
// @Success 200 {object} model.Peer
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/iface/{iface}/prepare [get]
func (e peerEndpoint) handlePrepareGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		interfaceId := c.Param("iface")
		if interfaceId == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing iface parameter"})
			return
		}

		peer, err := e.app.PreparePeer(ctx, domain.InterfaceIdentifier(interfaceId))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewPeer(peer))
	}
}

// handleCreatePost returns a gorm handler function.
//
// @ID peers_handleCreatePost
// @Tags Peer
// @Summary Prepare a new peer for the given interface.
// @Produce json
// @Param iface path string true "The interface identifier"
// @Param request body model.Peer true "The peer data"
// @Success 200 {object} model.Peer
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/iface/{iface}/new [post]
func (e peerEndpoint) handleCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		interfaceId := c.Param("iface")
		if interfaceId == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing iface parameter"})
			return
		}

		var p model.Peer
		err := c.BindJSON(&p)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if p.InterfaceIdentifier != interfaceId {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "interface id mismatch"})
			return
		}

		newPeer, err := e.app.CreatePeer(ctx, model.NewDomainPeer(&p))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewPeer(newPeer))
	}
}

// handleUpdatePut returns a gorm handler function.
//
// @ID peers_handleUpdatePut
// @Tags Peer
// @Summary Update the given peer record.
// @Produce json
// @Param id path string true "The peer identifier"
// @Param request body model.Peer true "The peer data"
// @Success 200 {object} model.Peer
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/{id} [post]
func (e peerEndpoint) handleUpdatePut() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		peerId := c.Param("id")
		if peerId == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing id parameter"})
			return
		}

		var p model.Peer
		err := c.BindJSON(&p)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if p.Identifier != peerId {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "peer id mismatch"})
			return
		}

		updatedPeer, err := e.app.UpdatePeer(ctx, model.NewDomainPeer(&p))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewPeer(updatedPeer))
	}
}

// handleDelete returns a gorm handler function.
//
// @ID peers_handleDelete
// @Tags Peer
// @Summary Delete the peer record.
// @Produce json
// @Param id path string true "The peer identifier"
// @Success 204 "No content if deletion was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/{id} [delete]
func (e peerEndpoint) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		err := e.app.DeletePeer(ctx, domain.PeerIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
