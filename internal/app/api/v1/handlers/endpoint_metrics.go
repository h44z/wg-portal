package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/h44z/wg-portal/internal/app/api/v1/models"
	"github.com/h44z/wg-portal/internal/domain"
)

type MetricsEndpointStatisticsService interface {
	GetForInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.InterfaceStatus, error)
	GetForUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, []domain.PeerStatus, error)
	GetForPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.PeerStatus, error)
}

type MetricsEndpoint struct {
	metrics MetricsEndpointStatisticsService
}

func NewMetricsEndpoint(metrics MetricsEndpointStatisticsService) *MetricsEndpoint {
	return &MetricsEndpoint{
		metrics: metrics,
	}
}

func (e MetricsEndpoint) GetName() string {
	return "MetricsEndpoint"
}

func (e MetricsEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/metrics", authenticator.LoggedIn())

	apiGroup.GET("/by-interface/:id", authenticator.LoggedIn(ScopeAdmin), e.handleMetricsForInterfaceGet())
	apiGroup.GET("/by-user/:id", authenticator.LoggedIn(), e.handleMetricsForUserGet())
	apiGroup.GET("/by-peer/:id", authenticator.LoggedIn(), e.handleMetricsForPeerGet())
}

// handleMetricsForInterfaceGet returns a gorm Handler function.
//
// @ID metrics_handleMetricsForInterfaceGet
// @Tags Metrics
// @Summary Get all metrics for a WireGuard Portal interface.
// @Param id path string true "The WireGuard interface identifier."
// @Produce json
// @Success 200 {object} models.InterfaceMetrics
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /metrics/by-interface/{id} [get]
// @Security BasicAuth
func (e MetricsEndpoint) handleMetricsForInterfaceGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		interfaceMetrics, err := e.metrics.GetForInterface(ctx, domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewInterfaceMetrics(interfaceMetrics))
	}
}

// handleMetricsForUserGet returns a gorm Handler function.
//
// @ID metrics_handleMetricsForUserGet
// @Tags Metrics
// @Summary Get all metrics for a WireGuard Portal user.
// @Param id path string true "The user identifier."
// @Produce json
// @Success 200 {object} models.UserMetrics
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /metrics/by-user/{id} [get]
// @Security BasicAuth
func (e MetricsEndpoint) handleMetricsForUserGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		user, userMetrics, err := e.metrics.GetForUser(ctx, domain.UserIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewUserMetrics(user, userMetrics))
	}
}

// handleMetricsForPeerGet returns a gorm Handler function.
//
// @ID metrics_handleMetricsForPeerGet
// @Tags Metrics
// @Summary Get all metrics for a WireGuard Portal peer.
// @Param id path string true "The peer identifier (public key)."
// @Produce json
// @Success 200 {object} models.PeerMetrics
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /metrics/by-peer/{id} [get]
// @Security BasicAuth
func (e MetricsEndpoint) handleMetricsForPeerGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		peerMetrics, err := e.metrics.GetForPeer(ctx, domain.PeerIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewPeerMetrics(peerMetrics))
	}
}
