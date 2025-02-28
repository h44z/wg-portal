package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/h44z/wg-portal/internal/app/api/v1/models"
	"github.com/h44z/wg-portal/internal/domain"
)

type ProvisioningEndpointProvisioningService interface {
	GetUserAndPeers(ctx context.Context, userId domain.UserIdentifier, email string) (
		*domain.User,
		[]domain.Peer,
		error,
	)
	GetPeerConfig(ctx context.Context, peerId domain.PeerIdentifier) ([]byte, error)
	GetPeerQrPng(ctx context.Context, peerId domain.PeerIdentifier) ([]byte, error)
	NewPeer(ctx context.Context, req models.ProvisioningRequest) (*domain.Peer, error)
}

type ProvisioningEndpoint struct {
	provisioning ProvisioningEndpointProvisioningService
}

func NewProvisioningEndpoint(provisioning ProvisioningEndpointProvisioningService) *ProvisioningEndpoint {
	return &ProvisioningEndpoint{
		provisioning: provisioning,
	}
}

func (e ProvisioningEndpoint) GetName() string {
	return "ProvisioningEndpoint"
}

func (e ProvisioningEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/provisioning", authenticator.LoggedIn())

	apiGroup.GET("/data/user-info", authenticator.LoggedIn(), e.handleUserInfoGet())
	apiGroup.GET("/data/peer-config", authenticator.LoggedIn(), e.handlePeerConfigGet())
	apiGroup.GET("/data/peer-qr", authenticator.LoggedIn(), e.handlePeerQrGet())

	apiGroup.POST("/new-peer", authenticator.LoggedIn(), e.handleNewPeerPost())
}

// handleUserInfoGet returns a gorm Handler function.
//
// @ID provisioning_handleUserInfoGet
// @Tags Provisioning
// @Summary Get information about all peer records for a given user.
// @Description Normal users can only access their own record. Admins can access all records.
// @Param UserId query string false "The user identifier that should be queried. If not set, the authenticated user is used."
// @Param Email query string false "The email address that should be queried. If UserId is set, this is ignored."
// @Produce json
// @Success 200 {object} models.UserInformation
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /provisioning/data/user-info [get]
// @Security BasicAuth
func (e ProvisioningEndpoint) handleUserInfoGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := strings.TrimSpace(c.Query("UserId"))
		email := strings.TrimSpace(c.Query("Email"))

		if id == "" && email == "" {
			id = string(domain.GetUserInfo(ctx).Id)
		}

		user, peers, err := e.provisioning.GetUserAndPeers(ctx, domain.UserIdentifier(id), email)
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewUserInformation(user, peers))
	}
}

// handlePeerConfigGet returns a gorm Handler function.
//
// @ID provisioning_handlePeerConfigGet
// @Tags Provisioning
// @Summary Get the peer configuration in wg-quick format.
// @Description Normal users can only access their own record. Admins can access all records.
// @Param PeerId query string true "The peer identifier (public key) that should be queried."
// @Produce plain
// @Produce json
// @Success 200 {string} string "The WireGuard configuration file"
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /provisioning/data/peer-config [get]
// @Security BasicAuth
func (e ProvisioningEndpoint) handlePeerConfigGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := strings.TrimSpace(c.Query("PeerId"))
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		peerConfig, err := e.provisioning.GetPeerConfig(ctx, domain.PeerIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.Data(http.StatusOK, "text/plain", peerConfig)
	}
}

// handlePeerQrGet returns a gorm Handler function.
//
// @ID provisioning_handlePeerQrGet
// @Tags Provisioning
// @Summary Get the peer configuration as QR code.
// @Description Normal users can only access their own record. Admins can access all records.
// @Param PeerId query string true "The peer identifier (public key) that should be queried."
// @Produce png
// @Produce json
// @Success 200 {file} binary "The WireGuard configuration QR code"
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /provisioning/data/peer-qr [get]
// @Security BasicAuth
func (e ProvisioningEndpoint) handlePeerQrGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := strings.TrimSpace(c.Query("PeerId"))
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		peerConfigQrCode, err := e.provisioning.GetPeerQrPng(ctx, domain.PeerIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.Data(http.StatusOK, "image/png", peerConfigQrCode)
	}
}

// handleNewPeerPost returns a gorm Handler function.
//
// @ID provisioning_handleNewPeerPost
// @Tags Provisioning
// @Summary Create a new peer for the given interface and user.
// @Description Normal users can only create new peers if self provisioning is allowed. Admins can always add new peers.
// @Param request body models.ProvisioningRequest true "Provisioning request model."
// @Produce json
// @Success 200 {object} models.Peer
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /provisioning/new-peer [post]
// @Security BasicAuth
func (e ProvisioningEndpoint) handleNewPeerPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		var req models.ProvisioningRequest
		err := c.BindJSON(&req)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		peer, err := e.provisioning.NewPeer(ctx, req)
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewPeer(peer))
	}
}
