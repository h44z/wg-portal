package handlers

import (
	"context"
	"io"
	"net/http"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal/app/api/core/request"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type PeerService interface {
	// GetInterfaceAndPeers returns the interface with the given id and all peers associated with it.
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	// PreparePeer returns a new peer with default values for the given interface.
	PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error)
	// GetPeer returns the peer with the given id.
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	// CreatePeer creates a new peer.
	CreatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error)
	// CreateMultiplePeers creates multiple new peers.
	CreateMultiplePeers(
		ctx context.Context,
		interfaceId domain.InterfaceIdentifier,
		r *domain.PeerCreationRequest,
	) ([]domain.Peer, error)
	// UpdatePeer updates the peer with the given id.
	UpdatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error)
	// DeletePeer deletes the peer with the given id.
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
	// GetPeerConfig returns the peer configuration for the given id.
	GetPeerConfig(ctx context.Context, id domain.PeerIdentifier, style string) (io.Reader, error)
	// GetPeerConfigQrCode returns the peer configuration as qr code for the given id.
	GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier, style string) (io.Reader, error)
	// SendPeerEmail sends the peer configuration via email.
	SendPeerEmail(ctx context.Context, linkOnly bool, style string, peers ...domain.PeerIdentifier) error
	// GetPeerStats returns the peer stats for the given interface.
	GetPeerStats(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.PeerStatus, error)
}

type PeerEndpoint struct {
	cfg           *config.Config
	peerService   PeerService
	authenticator Authenticator
	validator     Validator
}

func NewPeerEndpoint(
	cfg *config.Config,
	authenticator Authenticator,
	validator Validator,
	peerService PeerService,
) PeerEndpoint {
	return PeerEndpoint{
		cfg:           cfg,
		peerService:   peerService,
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

		_, peers, err := e.peerService.GetInterfaceAndPeers(r.Context(), domain.InterfaceIdentifier(interfaceId))
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

		peer, err := e.peerService.GetPeer(r.Context(), domain.PeerIdentifier(peerId))
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

		peer, err := e.peerService.PreparePeer(r.Context(), domain.InterfaceIdentifier(interfaceId))
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

		newPeer, err := e.peerService.CreatePeer(r.Context(), model.NewDomainPeer(&p))
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

		newPeers, err := e.peerService.CreateMultiplePeers(r.Context(), domain.InterfaceIdentifier(interfaceId),
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

		updatedPeer, err := e.peerService.UpdatePeer(r.Context(), model.NewDomainPeer(&p))
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

		err := e.peerService.DeletePeer(r.Context(), domain.PeerIdentifier(id))
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
// @Param style query string false "The configuration style"
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

		configStyle := e.getConfigStyle(r)

		configTxt, err := e.peerService.GetPeerConfig(r.Context(), domain.PeerIdentifier(id), configStyle)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		configTxtString, err := io.ReadAll(configTxt)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.JSON(w, http.StatusOK, string(configTxtString))
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
// @Param style query string false "The configuration style"
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

		configStyle := e.getConfigStyle(r)

		configQr, err := e.peerService.GetPeerConfigQrCode(r.Context(), domain.PeerIdentifier(id), configStyle)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		configQrData, err := io.ReadAll(configQr)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		respond.Data(w, http.StatusOK, "image/png", configQrData)
	}
}

// handleEmailPost returns a gorm Handler function.
//
// @ID peers_handleEmailPost
// @Tags Peer
// @Summary Send peer configuration via email.
// @Produce json
// @Param request body model.PeerMailRequest true "The peer mail request data"
// @Param style query string false "The configuration style"
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

		configStyle := e.getConfigStyle(r)

		peerIds := make([]domain.PeerIdentifier, len(req.Identifiers))
		for i := range req.Identifiers {
			peerIds[i] = domain.PeerIdentifier(req.Identifiers[i])
		}
		if err := e.peerService.SendPeerEmail(r.Context(), req.LinkOnly, configStyle, peerIds...); err != nil {
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

		stats, err := e.peerService.GetPeerStats(r.Context(), domain.InterfaceIdentifier(interfaceId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeerStats(e.cfg.Statistics.CollectPeerData, stats))
	}
}

func (e PeerEndpoint) getConfigStyle(r *http.Request) string {
	configStyle := request.QueryDefault(r, "style", domain.ConfigStyleWgQuick)
	if configStyle != domain.ConfigStyleWgQuick && configStyle != domain.ConfigStyleRaw {
		configStyle = domain.ConfigStyleWgQuick // default to wg-quick style
	}
	return configStyle
}
