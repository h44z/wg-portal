package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app/api/v1/models"
	"github.com/h44z/wg-portal/internal/domain"
)

type PeerService interface {
	GetForInterface(context.Context, domain.InterfaceIdentifier) ([]domain.Peer, error)
	GetForUser(context.Context, domain.UserIdentifier) ([]domain.Peer, error)
	GetById(context.Context, domain.PeerIdentifier) (*domain.Peer, error)
	Create(context.Context, *domain.Peer) (*domain.Peer, error)
	Update(context.Context, domain.PeerIdentifier, *domain.Peer) (*domain.Peer, error)
	Delete(context.Context, domain.PeerIdentifier) error
}

type PeerEndpoint struct {
	peers PeerService
}

func NewPeerEndpoint(peerService PeerService) *PeerEndpoint {
	return &PeerEndpoint{
		peers: peerService,
	}
}

func (e PeerEndpoint) GetName() string {
	return "PeerEndpoint"
}

func (e PeerEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/peer", authenticator.LoggedIn())

	apiGroup.GET("/by-interface/:id", authenticator.LoggedIn(ScopeAdmin), e.handleAllForInterfaceGet())
	apiGroup.GET("/by-user/:id", authenticator.LoggedIn(), e.handleAllForUserGet())
	apiGroup.GET("/by-id/:id", authenticator.LoggedIn(), e.handleByIdGet())

	apiGroup.POST("/new", authenticator.LoggedIn(ScopeAdmin), e.handleCreatePost())
	apiGroup.PUT("/by-id/:id", authenticator.LoggedIn(ScopeAdmin), e.handleUpdatePut())
	apiGroup.DELETE("/by-id/:id", authenticator.LoggedIn(ScopeAdmin), e.handleDelete())
}

// handleAllForInterfaceGet returns a gorm Handler function.
//
// @ID peers_handleAllForInterfaceGet
// @Tags Peers
// @Summary Get all peer records for a given WireGuard interface.
// @Param id path string true "The WireGuard interface identifier."
// @Produce json
// @Success 200 {object} []models.Peer
// @Failure 401 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /peer/by-interface/{id} [get]
// @Security BasicAuth
func (e PeerEndpoint) handleAllForInterfaceGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		interfacePeers, err := e.peers.GetForInterface(ctx, domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewPeers(interfacePeers))
	}
}

// handleAllForUserGet returns a gorm Handler function.
//
// @ID peers_handleAllForUserGet
// @Tags Peers
// @Summary Get all peer records for a given user.
// @Description Normal users can only access their own records. Admins can access all records.
// @Param id path string true "The user identifier."
// @Produce json
// @Success 200 {object} []models.Peer
// @Failure 401 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /peer/by-user/{id} [get]
// @Security BasicAuth
func (e PeerEndpoint) handleAllForUserGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		interfacePeers, err := e.peers.GetForUser(ctx, domain.UserIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewPeers(interfacePeers))
	}
}

// handleByIdGet returns a gorm Handler function.
//
// @ID peers_handleByIdGet
// @Tags Peers
// @Summary Get a specific peer record by its identifier (public key).
// @Description Normal users can only access their own records. Admins can access all records.
// @Param id path string true "The peer identifier (public key)."
// @Produce json
// @Success 200 {object} models.Peer
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /peer/by-id/{id} [get]
// @Security BasicAuth
func (e PeerEndpoint) handleByIdGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		peer, err := e.peers.GetById(ctx, domain.PeerIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewPeer(peer))
	}
}

// handleCreatePost returns a gorm handler function.
//
// @ID peers_handleCreatePost
// @Tags Peers
// @Summary Create a new peer record.
// @Description Only admins can create new records.
// @Param request body models.Peer true "The peer data."
// @Produce json
// @Success 200 {object} models.Peer
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 409 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /peer/new [post]
// @Security BasicAuth
func (e PeerEndpoint) handleCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		var peer models.Peer
		err := c.BindJSON(&peer)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newPeer, err := e.peers.Create(ctx, models.NewDomainPeer(&peer))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewPeer(newPeer))
	}
}

// handleUpdatePut returns a gorm handler function.
//
// @ID peers_handleUpdatePut
// @Tags Peers
// @Summary Update a peer record.
// @Description Only admins can update existing records.
// @Param id path string true "The peer identifier."
// @Param request body models.Peer true "The peer data."
// @Produce json
// @Success 200 {object} models.Peer
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /peer/by-id/{id} [put]
// @Security BasicAuth
func (e PeerEndpoint) handleUpdatePut() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		var peer models.Peer
		err := c.BindJSON(&peer)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		updatedPeer, err := e.peers.Update(ctx, domain.PeerIdentifier(id), models.NewDomainPeer(&peer))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewPeer(updatedPeer))
	}
}

// handleDelete returns a gorm handler function.
//
// @ID peers_handleDelete
// @Tags Peers
// @Summary Delete the peer record.
// @Param id path string true "The peer identifier."
// @Produce json
// @Success 204 "No content if deletion was successful."
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /peer/by-id/{id} [delete]
// @Security BasicAuth
func (e PeerEndpoint) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		err := e.peers.Delete(ctx, domain.PeerIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.Status(http.StatusNoContent)
	}
}
