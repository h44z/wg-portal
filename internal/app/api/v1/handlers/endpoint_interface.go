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

type InterfaceEndpointInterfaceService interface {
	GetAll(context.Context) ([]domain.Interface, [][]domain.Peer, error)
	GetById(context.Context, domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	Prepare(context.Context) (*domain.Interface, error)
	Create(context.Context, *domain.Interface) (*domain.Interface, error)
	Update(context.Context, domain.InterfaceIdentifier, *domain.Interface) (*domain.Interface, []domain.Peer, error)
	Delete(context.Context, domain.InterfaceIdentifier) error
}

type InterfaceEndpoint struct {
	interfaces    InterfaceEndpointInterfaceService
	authenticator Authenticator
	validator     Validator
}

func NewInterfaceEndpoint(
	authenticator Authenticator,
	validator Validator,
	interfaceService InterfaceEndpointInterfaceService,
) *InterfaceEndpoint {
	return &InterfaceEndpoint{
		authenticator: authenticator,
		validator:     validator,
		interfaces:    interfaceService,
	}
}

func (e InterfaceEndpoint) GetName() string {
	return "InterfaceEndpoint"
}

func (e InterfaceEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/interface")
	apiGroup.Use(e.authenticator.LoggedIn(ScopeAdmin))

	apiGroup.HandleFunc("GET /all", e.handleAllGet())
	apiGroup.HandleFunc("GET /by-id/{id}", e.handleByIdGet())

	apiGroup.HandleFunc("GET /prepare", e.handlePrepareGet())
	apiGroup.HandleFunc("POST /new", e.handleCreatePost())
	apiGroup.HandleFunc("PUT /by-id/{id}", e.handleUpdatePut())
	apiGroup.HandleFunc("DELETE /by-id/{id}", e.handleDelete())
}

// handleAllGet returns a gorm Handler function.
//
// @ID interface_handleAllGet
// @Tags Interfaces
// @Summary Get all interface records.
// @Produce json
// @Success 200 {object} []models.Interface
// @Failure 401 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/all [get]
// @Security BasicAuth
func (e InterfaceEndpoint) handleAllGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allInterfaces, allPeersPerInterface, err := e.interfaces.GetAll(r.Context())
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewInterfaces(allInterfaces, allPeersPerInterface))
	}
}

// handleByIdGet returns a gorm Handler function.
//
// @ID interfaces_handleByIdGet
// @Tags Interfaces
// @Summary Get a specific interface record by its identifier.
// @Param id path string true "The interface identifier."
// @Produce json
// @Success 200 {object} models.Interface
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/by-id/{id} [get]
// @Security BasicAuth
func (e InterfaceEndpoint) handleByIdGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		iface, interfacePeers, err := e.interfaces.GetById(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewInterface(iface, interfacePeers))
	}
}

// handlePrepareGet returns a gorm handler function.
//
// @ID interfaces_handlePrepareGet
// @Tags Interfaces
// @Summary Prepare a new interface record.
// @Description This endpoint returns a new interface with default values (fresh key pair, valid name, new IP address pool, ...).
// @Produce json
// @Success 200 {object} models.Interface
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/prepare [get]
// @Security BasicAuth
func (e InterfaceEndpoint) handlePrepareGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		iface, err := e.interfaces.Prepare(r.Context())
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewInterface(iface, nil))
	}
}

// handleCreatePost returns a gorm handler function.
//
// @ID interfaces_handleCreatePost
// @Tags Interfaces
// @Summary Create a new interface record.
// @Description This endpoint creates a new interface with the provided data. All required fields must be filled (e.g. name, private key, public key, ...).
// @Param request body models.Interface true "The interface data."
// @Produce json
// @Success 200 {object} models.Interface
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 409 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/new [post]
// @Security BasicAuth
func (e InterfaceEndpoint) handleCreatePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var iface models.Interface
		if err := request.BodyJson(r, &iface); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(iface); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newInterface, err := e.interfaces.Create(r.Context(), models.NewDomainInterface(&iface))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewInterface(newInterface, nil))
	}
}

// handleUpdatePut returns a gorm handler function.
//
// @ID interfaces_handleUpdatePut
// @Tags Interfaces
// @Summary Update an interface record.
// @Description This endpoint updates an existing interface with the provided data. All required fields must be filled (e.g. name, private key, public key, ...).
// @Param id path string true "The interface identifier."
// @Param request body models.Interface true "The interface data."
// @Produce json
// @Success 200 {object} models.Interface
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/by-id/{id} [put]
// @Security BasicAuth
func (e InterfaceEndpoint) handleUpdatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		var iface models.Interface
		if err := request.BodyJson(r, &iface); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(iface); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != iface.Identifier {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "interface id mismatch"})
			return
		}

		updatedInterface, updatedInterfacePeers, err := e.interfaces.Update(
			r.Context(),
			domain.InterfaceIdentifier(id),
			models.NewDomainInterface(&iface),
		)
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewInterface(updatedInterface, updatedInterfacePeers))
	}
}

// handleDelete returns a gorm handler function.
//
// @ID interfaces_handleDelete
// @Tags Interfaces
// @Summary Delete the interface record.
// @Param id path string true "The interface identifier."
// @Produce json
// @Success 204 "No content if deletion was successful."
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/by-id/{id} [delete]
// @Security BasicAuth
func (e InterfaceEndpoint) handleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		err := e.interfaces.Delete(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}
