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

type PeerEndpoint struct {
	app           *app.App
	authenticator Authenticator
	validator     Validator
}

func NewPeerEndpoint(app *app.App, authenticator Authenticator, validator Validator) PeerEndpoint {
	return PeerEndpoint{
		app:           app,
		authenticator: authenticator,
		validator:     validator,
	}
}

func (e PeerEndpoint) GetName() string {
	return "PeerEndpoint"
}

func (e PeerEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/peer")
	apiGroup.Use(e.authenticator.LoggedIn())

	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("GET /iface/{iface}/all", e.handleAllGet())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("GET /iface/{iface}/stats", e.handleStatsGet())
	apiGroup.HandleFunc("GET /iface/{iface}/prepare", e.handlePrepareGet())
	apiGroup.HandleFunc("POST /iface/{iface}/new", e.handleCreatePost())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /iface/{iface}/multiplenew",
		e.handleCreateMultiplePost())
	apiGroup.HandleFunc("GET /config-qr/{id}", e.handleQrCodeGet())
	apiGroup.HandleFunc("POST /config-mail", e.handleEmailPost())
	apiGroup.HandleFunc("GET /config/{id}", e.handleConfigGet())
	apiGroup.HandleFunc("GET /{id}", e.handleSingleGet())
	apiGroup.HandleFunc("PUT /{id}", e.handleUpdatePut())
	apiGroup.HandleFunc("DELETE /{id}", e.handleDelete())
}

// handleAllGet returns a gorm Handler function.
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
func (e PeerEndpoint) handleAllGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interfaceId := Base64UrlDecode(request.Path(r, "iface"))
		if interfaceId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing iface parameter"})
			return
		}

		_, peers, err := e.app.GetInterfaceAndPeers(r.Context(), domain.InterfaceIdentifier(interfaceId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeers(peers))
	}
}

// handleSingleGet returns a gorm Handler function.
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
func (e PeerEndpoint) handleSingleGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		peerId := Base64UrlDecode(request.Path(r, "id"))
		if peerId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing id parameter"})
			return
		}

		peer, err := e.app.GetPeer(r.Context(), domain.PeerIdentifier(peerId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeer(peer))
	}
}

// handlePrepareGet returns a gorm Handler function.
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
func (e PeerEndpoint) handlePrepareGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interfaceId := Base64UrlDecode(request.Path(r, "iface"))
		if interfaceId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing iface parameter"})
			return
		}

		peer, err := e.app.PreparePeer(r.Context(), domain.InterfaceIdentifier(interfaceId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeer(peer))
	}
}

// handleCreatePost returns a gorm Handler function.
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
func (e PeerEndpoint) handleCreatePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interfaceId := Base64UrlDecode(request.Path(r, "iface"))
		if interfaceId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing iface parameter"})
			return
		}

		var p model.Peer
		if err := request.BodyJson(r, &p); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(p); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if p.InterfaceIdentifier != interfaceId {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "interface id mismatch"})
			return
		}

		newPeer, err := e.app.CreatePeer(r.Context(), model.NewDomainPeer(&p))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeer(newPeer))
	}
}

// handleCreateMultiplePost returns a gorm Handler function.
//
// @ID peers_handleCreateMultiplePost
// @Tags Peer
// @Summary Create multiple new peers for the given interface.
// @Produce json
// @Param iface path string true "The interface identifier"
// @Param request body model.MultiPeerRequest true "The peer creation request data"
// @Success 200 {object} []model.Peer
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/iface/{iface}/multiplenew [post]
func (e PeerEndpoint) handleCreateMultiplePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interfaceId := Base64UrlDecode(request.Path(r, "iface"))
		if interfaceId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing iface parameter"})
			return
		}

		var req model.MultiPeerRequest
		if err := request.BodyJson(r, &req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newPeers, err := e.app.CreateMultiplePeers(r.Context(), domain.InterfaceIdentifier(interfaceId),
			model.NewDomainPeerCreationRequest(&req))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeers(newPeers))
	}
}

// handleUpdatePut returns a gorm Handler function.
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
// @Router /peer/{id} [put]
func (e PeerEndpoint) handleUpdatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		peerId := Base64UrlDecode(request.Path(r, "id"))
		if peerId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing id parameter"})
			return
		}

		var p model.Peer
		if err := request.BodyJson(r, &p); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(p); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if p.Identifier != peerId {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "peer id mismatch"})
			return
		}

		updatedPeer, err := e.app.UpdatePeer(r.Context(), model.NewDomainPeer(&p))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeer(updatedPeer))
	}
}

// handleDelete returns a gorm Handler function.
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
func (e PeerEndpoint) handleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing peer id"})
			return
		}

		err := e.app.DeletePeer(r.Context(), domain.PeerIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleConfigGet returns a gorm Handler function.
//
// @ID peers_handleConfigGet
// @Tags Peer
// @Summary Get peer configuration as string.
// @Produce json
// @Param id path string true "The peer identifier"
// @Success 200 {object} string
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/config/{id} [get]
func (e PeerEndpoint) handleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		config, err := e.app.GetPeerConfig(r.Context(), domain.PeerIdentifier(id))
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

// handleQrCodeGet returns a gorm Handler function.
//
// @ID peers_handleQrCodeGet
// @Tags Peer
// @Summary Get peer configuration as qr code.
// @Produce png
// @Produce json
// @Param id path string true "The peer identifier"
// @Success 200 {file} binary
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/config-qr/{id} [get]
func (e PeerEndpoint) handleQrCodeGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		config, err := e.app.GetPeerConfigQrCode(r.Context(), domain.PeerIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		configData, err := io.ReadAll(config)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.Data(w, http.StatusOK, "image/png", configData)
	}
}

// handleEmailPost returns a gorm Handler function.
//
// @ID peers_handleEmailPost
// @Tags Peer
// @Summary Send peer configuration via email.
// @Produce json
// @Param request body model.PeerMailRequest true "The peer mail request data"
// @Success 204 "No content if mail sending was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/config-mail [post]
func (e PeerEndpoint) handleEmailPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.PeerMailRequest
		if err := request.BodyJson(r, &req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if len(req.Identifiers) == 0 {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing peer identifiers"})
			return
		}

		peerIds := make([]domain.PeerIdentifier, len(req.Identifiers))
		for i := range req.Identifiers {
			peerIds[i] = domain.PeerIdentifier(req.Identifiers[i])
		}
		if err := e.app.SendPeerEmail(r.Context(), req.LinkOnly, peerIds...); err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleStatsGet returns a gorm Handler function.
//
// @ID peers_handleStatsGet
// @Tags Peer
// @Summary Get peer stats for the given interface.
// @Produce json
// @Param iface path string true "The interface identifier"
// @Success 200 {object} model.PeerStats
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /peer/iface/{iface}/stats [get]
func (e PeerEndpoint) handleStatsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interfaceId := Base64UrlDecode(request.Path(r, "iface"))
		if interfaceId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "missing iface parameter"})
			return
		}

		stats, err := e.app.GetPeerStats(r.Context(), domain.InterfaceIdentifier(interfaceId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeerStats(e.app.Config.Statistics.CollectPeerData, stats))
	}
}
