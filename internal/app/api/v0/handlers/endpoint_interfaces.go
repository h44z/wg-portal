package handlers

import (
	"context"
	"io"
	"net/http"

	"github.com/go-pkgz/routegroup"

	"github.com/fedor-git/wg-portal-2/internal/app/api/core/request"
	"github.com/fedor-git/wg-portal-2/internal/app/api/core/respond"
	"github.com/fedor-git/wg-portal-2/internal/app/api/v0/model"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type InterfaceService interface {
	// GetInterfaceAndPeers returns the interface with the given id and all peers associated with it.
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	// PrepareInterface returns a new interface with default values.
	PrepareInterface(ctx context.Context) (*domain.Interface, error)
	// CreateInterface creates a new interface.
	CreateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error)
	// UpdateInterface updates the interface with the given id.
	UpdateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, []domain.Peer, error)
	// DeleteInterface deletes the interface with the given id.
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
	// GetAllInterfacesAndPeers returns all interfaces and all peers associated with them.
	GetAllInterfacesAndPeers(ctx context.Context) ([]domain.Interface, [][]domain.Peer, error)
	// GetInterfaceConfig returns the interface configuration as string.
	GetInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) (io.Reader, error)
	// PersistInterfaceConfig persists the interface configuration to a file.
	PersistInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) error
	// ApplyPeerDefaults applies the peer defaults to all peers of the given interface.
	ApplyPeerDefaults(ctx context.Context, in *domain.Interface) error
}

type InterfaceEndpoint struct {
	cfg              *config.Config
	interfaceService InterfaceService
	authenticator    Authenticator
	validator        Validator
}

func NewInterfaceEndpoint(
	cfg *config.Config,
	authenticator Authenticator,
	validator Validator,
	interfaceService InterfaceService,
) InterfaceEndpoint {
	return InterfaceEndpoint{
		cfg:              cfg,
		interfaceService: interfaceService,
		authenticator:    authenticator,
		validator:        validator,
	}
}

func (e InterfaceEndpoint) GetName() string {
	return "InterfaceEndpoint"
}

func (e InterfaceEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/interface")
	apiGroup.Use(e.authenticator.LoggedIn(ScopeAdmin))

	apiGroup.HandleFunc("GET /prepare", e.handlePrepareGet())
	apiGroup.HandleFunc("GET /all", e.handleAllGet())
	apiGroup.HandleFunc("GET /get/{id}", e.handleSingleGet())
	apiGroup.HandleFunc("PUT /{id}", e.handleUpdatePut())
	apiGroup.HandleFunc("DELETE /{id}", e.handleDelete())
	apiGroup.HandleFunc("POST /new", e.handleCreatePost())
	apiGroup.HandleFunc("GET /config/{id}", e.handleConfigGet())
	apiGroup.HandleFunc("POST /{id}/save-config", e.handleSaveConfigPost())
	apiGroup.HandleFunc("POST /{id}/apply-peer-defaults", e.handleApplyPeerDefaultsPost())

	apiGroup.HandleFunc("GET /peers/{id}", e.handlePeersGet())
}

// handlePrepareGet returns a gorm Handler function.
//
// @ID interfaces_handlePrepareGet
// @Tags Interface
// @Summary Prepare a new interface.
// @Produce json
// @Success 200 {object} model.Interface
// @Failure 500 {object} model.Error
// @Router /interface/prepare [get]
func (e InterfaceEndpoint) handlePrepareGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := e.interfaceService.PrepareInterface(r.Context())
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewInterface(in, nil))
	}
}

// handleAllGet returns a gorm Handler function.
//
// @ID interfaces_handleAllGet
// @Tags Interface
// @Summary Get all available interfaces.
// @Produce json
// @Success 200 {object} []model.Interface
// @Failure 500 {object} model.Error
// @Router /interface/all [get]
func (e InterfaceEndpoint) handleAllGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interfaces, peers, err := e.interfaceService.GetAllInterfacesAndPeers(r.Context())
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewInterfaces(interfaces, peers))
	}
}

// handleSingleGet returns a gorm Handler function.
//
// @ID interfaces_handleSingleGet
// @Tags Interface
// @Summary Get single interface.
// @Produce json
// @Success 200 {object} model.Interface
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/get/{id} [get]
func (e InterfaceEndpoint) handleSingleGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		iface, peers, err := e.interfaceService.GetInterfaceAndPeers(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewInterface(iface, peers))
	}
}

// handleConfigGet returns a gorm Handler function.
//
// @ID interfaces_handleConfigGet
// @Tags Interface
// @Summary Get interface configuration as string.
// @Produce json
// @Success 200 {object} string
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/config/{id} [get]
func (e InterfaceEndpoint) handleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		config, err := e.interfaceService.GetInterfaceConfig(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		configString, err := io.ReadAll(config)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, string(configString))
	}
}

// handleUpdatePut returns a gorm Handler function.
//
// @ID interfaces_handleUpdatePut
// @Tags Interface
// @Summary Update the interface record.
// @Produce json
// @Param id path string true "The interface identifier"
// @Param request body model.Interface true "The interface data"
// @Success 200 {object} model.Interface
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/{id} [put]
func (e InterfaceEndpoint) handleUpdatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		var in model.Interface
		if err := request.BodyJson(r, &in); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(in); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != in.Identifier {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "interface id mismatch"})
			return
		}

		updatedInterface, peers, err := e.interfaceService.UpdateInterface(r.Context(), model.NewDomainInterface(&in))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewInterface(updatedInterface, peers))
	}
}

// handleCreatePost returns a gorm Handler function.
//
// @ID interfaces_handleCreatePost
// @Tags Interface
// @Summary Create the new interface record.
// @Produce json
// @Param request body model.Interface true "The interface data"
// @Success 200 {object} model.Interface
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/new [post]
func (e InterfaceEndpoint) handleCreatePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in model.Interface
		if err := request.BodyJson(r, &in); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(in); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newInterface, err := e.interfaceService.CreateInterface(r.Context(), model.NewDomainInterface(&in))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewInterface(newInterface, nil))
	}
}

// handlePeersGet returns a gorm Handler function.
//
// @ID interfaces_handlePeersGet
// @Tags Interface
// @Summary Get peers for the given interface.
// @Produce json
// @Success 200 {object} []model.Peer
// @Failure 500 {object} model.Error
// @Router /interface/peers/{id} [get]
func (e InterfaceEndpoint) handlePeersGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		_, peers, err := e.interfaceService.GetInterfaceAndPeers(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeers(peers))
	}
}

// handleDelete returns a gorm Handler function.
//
// @ID interfaces_handleDelete
// @Tags Interface
// @Summary Delete the interface record.
// @Produce json
// @Param id path string true "The interface identifier"
// @Success 204 "No content if deletion was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/{id} [delete]
func (e InterfaceEndpoint) handleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		err := e.interfaceService.DeleteInterface(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleSaveConfigPost returns a gorm Handler function.
//
// @ID interfaces_handleSaveConfigPost
// @Tags Interface
// @Summary Save the interface configuration in wg-quick format to a file.
// @Produce json
// @Param id path string true "The interface identifier"
// @Success 204 "No content if saving the configuration was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/{id}/save-config [post]
func (e InterfaceEndpoint) handleSaveConfigPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		err := e.interfaceService.PersistInterfaceConfig(r.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleApplyPeerDefaultsPost returns a gorm Handler function.
//
// @ID interfaces_handleApplyPeerDefaultsPost
// @Tags Interface
// @Summary Apply all peer defaults to the available peers.
// @Produce json
// @Param id path string true "The interface identifier"
// @Param request body model.Interface true "The interface data"
// @Success 204 "No content if applying peer defaults was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/{id}/apply-peer-defaults [post]
func (e InterfaceEndpoint) handleApplyPeerDefaultsPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		var in model.Interface
		if err := request.BodyJson(r, &in); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(in); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != in.Identifier {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "interface id mismatch"})
			return
		}

		if err := e.interfaceService.ApplyPeerDefaults(r.Context(), model.NewDomainInterface(&in)); err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}
