package handlers

import (
	"context"
	"net/http"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal/app/api/core/request"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v1/models"
	"github.com/h44z/wg-portal/internal/domain"
)

type MetricsEndpointStatisticsService interface {
	GetForInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.InterfaceStatus, error)
	GetForUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, []domain.PeerStatus, error)
	GetForPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.PeerStatus, error)
}

type MetricsEndpoint struct {
	metrics       MetricsEndpointStatisticsService
	authenticator Authenticator
	validator     Validator
}

func NewMetricsEndpoint(
	authenticator Authenticator,
	validator Validator,
	metrics MetricsEndpointStatisticsService,
) *MetricsEndpoint {
	return &MetricsEndpoint{
		authenticator: authenticator,
		validator:     validator,
		metrics:       metrics,
	}
}

func (e MetricsEndpoint) GetName() string {
	return "MetricsEndpoint"
}

func (e MetricsEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/metrics")
	apiGroup.Use(e.authenticator.LoggedIn())

	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("GET /by-interface/{id}",
		e.handleMetricsForInterfaceGet())
	apiGroup.HandleFunc("GET /by-user/{id}", e.handleMetricsForUserGet())
	apiGroup.HandleFunc("GET /by-peer/{id}", e.handleMetricsForPeerGet())
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
func (e MetricsEndpoint) handleMetricsForInterfaceGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		interfaceMetrics, err := e.metrics.GetForInterface(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewInterfaceMetrics(interfaceMetrics))
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
func (e MetricsEndpoint) handleMetricsForUserGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		user, userMetrics, err := e.metrics.GetForUser(r.Context(), domain.UserIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewUserMetrics(user, userMetrics))
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
func (e MetricsEndpoint) handleMetricsForPeerGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		peerMetrics, err := e.metrics.GetForPeer(r.Context(), domain.PeerIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewPeerMetrics(peerMetrics))
	}
}
