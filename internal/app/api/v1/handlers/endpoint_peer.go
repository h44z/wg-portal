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

type PeerService interface {
	GetForInterface(context.Context, domain.InterfaceIdentifier) ([]domain.Peer, error)
	GetForUser(context.Context, domain.UserIdentifier) ([]domain.Peer, error)
	GetById(context.Context, domain.PeerIdentifier) (*domain.Peer, error)
	Prepare(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error)
	Create(context.Context, *domain.Peer) (*domain.Peer, error)
	Update(context.Context, domain.PeerIdentifier, *domain.Peer) (*domain.Peer, error)
	Delete(context.Context, domain.PeerIdentifier) error
}

type PeerEndpoint struct {
	peers         PeerService
	authenticator Authenticator
	validator     Validator
}

func NewPeerEndpoint(
	authenticator Authenticator,
	validator Validator, peerService PeerService,
) *PeerEndpoint {
	return &PeerEndpoint{
		authenticator: authenticator,
		validator:     validator,
		peers:         peerService,
	}
}

func (e PeerEndpoint) GetName() string {
	return "PeerEndpoint"
}

func (e PeerEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/peer")
	apiGroup.Use(e.authenticator.LoggedIn())

	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("GET /by-interface/{id}",
		e.handleAllForInterfaceGet())
	apiGroup.HandleFunc("GET /by-user/{id}", e.handleAllForUserGet())
	apiGroup.HandleFunc("GET /by-id/{id}", e.handleByIdGet())

	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("GET /prepare/{id}", e.handlePrepareGet())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /new", e.handleCreatePost())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("PUT /by-id/{id}", e.handleUpdatePut())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("DELETE /by-id/{id}", e.handleDelete())
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
func (e PeerEndpoint) handleAllForInterfaceGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		interfacePeers, err := e.peers.GetForInterface(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewPeers(interfacePeers))
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
func (e PeerEndpoint) handleAllForUserGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		interfacePeers, err := e.peers.GetForUser(r.Context(), domain.UserIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewPeers(interfacePeers))
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
func (e PeerEndpoint) handleByIdGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		peer, err := e.peers.GetById(r.Context(), domain.PeerIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewPeer(peer))
	}
}

// handlePrepareGet returns a gorm handler function.
//
// @ID peers_handlePrepareGet
// @Tags Peers
// @Summary Prepare a new peer record for the given WireGuard interface.
// @Description This endpoint is used to prepare a new peer record. The returned data contains a fresh key pair and valid ip address.
// @Param id path string true "The interface identifier."
// @Produce json
// @Success 200 {object} models.Peer
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /peer/prepare/{id} [get]
// @Security BasicAuth
func (e PeerEndpoint) handlePrepareGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		peer, err := e.peers.Prepare(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewPeer(peer))
	}
}

// handleCreatePost returns a gorm handler function.
//
// @ID peers_handleCreatePost
// @Tags Peers
// @Summary Create a new peer record.
// @Description Only admins can create new records. The peer record must contain all required fields (e.g., public key, allowed IPs).
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
func (e PeerEndpoint) handleCreatePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var peer models.Peer
		if err := request.BodyJson(r, &peer); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(peer); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newPeer, err := e.peers.Create(r.Context(), models.NewDomainPeer(&peer))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewPeer(newPeer))
	}
}

// handleUpdatePut returns a gorm handler function.
//
// @ID peers_handleUpdatePut
// @Tags Peers
// @Summary Update a peer record.
// @Description Only admins can update existing records. The peer record must contain all required fields (e.g., public key, allowed IPs).
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
func (e PeerEndpoint) handleUpdatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		var peer models.Peer
		if err := request.BodyJson(r, &peer); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(peer); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		updatedPeer, err := e.peers.Update(r.Context(), domain.PeerIdentifier(id), models.NewDomainPeer(&peer))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewPeer(updatedPeer))
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
func (e PeerEndpoint) handleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		err := e.peers.Delete(r.Context(), domain.PeerIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}
