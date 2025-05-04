package handlers

import (
	"context"
	"net/http"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type AuditService interface {
	// GetAll returns all audit entries ordered by timestamp. Newest first.
	GetAll(ctx context.Context) ([]domain.AuditEntry, error)
}

type AuditEndpoint struct {
	cfg           *config.Config
	authenticator Authenticator
	auditService  AuditService
}

func NewAuditEndpoint(
	cfg *config.Config,
	authenticator Authenticator,
	auditService AuditService,
) AuditEndpoint {
	return AuditEndpoint{
		cfg:           cfg,
		authenticator: authenticator,
		auditService:  auditService,
	}
}

func (e AuditEndpoint) GetName() string {
	return "AuditEndpoint"
}

func (e AuditEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/audit")
	apiGroup.Use(e.authenticator.LoggedIn(ScopeAdmin))

	apiGroup.HandleFunc("GET /entries", e.handleEntriesGet())
}

// handleExternalLoginProvidersGet returns a gorm Handler function.
//
// @ID audit_handleEntriesGet
// @Tags Audit
// @Summary Get all available audit entries. Ordered by timestamp.
// @Produce json
// @Success 200 {object} []model.AuditEntry
// @Router /audit/entries [get]
func (e AuditEndpoint) handleEntriesGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providers, err := e.auditService.GetAll(r.Context())
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewAuditEntries(providers))
	}
}
