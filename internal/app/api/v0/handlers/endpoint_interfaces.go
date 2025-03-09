package handlers

import (
	"io"
	"net/http"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/core/request"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
)

type InterfaceEndpoint struct {
	app           *app.App
	authenticator Authenticator
	validator     Validator
}

func NewInterfaceEndpoint(app *app.App, authenticator Authenticator, validator Validator) InterfaceEndpoint {
	return InterfaceEndpoint{
		app:           app,
		authenticator: authenticator,
		validator:     validator,
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
		in, err := e.app.PrepareInterface(r.Context())
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
		interfaces, peers, err := e.app.GetAllInterfacesAndPeers(r.Context())
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

		iface, peers, err := e.app.GetInterfaceAndPeers(r.Context(), domain.InterfaceIdentifier(id))
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

		config, err := e.app.GetInterfaceConfig(r.Context(), domain.InterfaceIdentifier(id))
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

		updatedInterface, peers, err := e.app.UpdateInterface(r.Context(), model.NewDomainInterface(&in))
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

		newInterface, err := e.app.CreateInterface(r.Context(), model.NewDomainInterface(&in))
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

		_, peers, err := e.app.GetInterfaceAndPeers(r.Context(), domain.InterfaceIdentifier(id))
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

		err := e.app.DeleteInterface(r.Context(), domain.InterfaceIdentifier(id))
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

		err := e.app.PersistInterfaceConfig(r.Context(), domain.InterfaceIdentifier(id))
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

		if err := e.app.ApplyPeerDefaults(r.Context(), model.NewDomainInterface(&in)); err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}
