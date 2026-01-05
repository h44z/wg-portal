package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal/app/api/core/request"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type UserService interface {
	// GetUser returns the user with the given id.
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	// GetAllUsers returns all users.
	GetAllUsers(ctx context.Context) ([]domain.User, error)
	// UpdateUser updates the user with the given id.
	UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	// CreateUser creates a new user.
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	// DeleteUser deletes the user with the given id.
	DeleteUser(ctx context.Context, id domain.UserIdentifier) error
	// ActivateApi enables the API for the user with the given id.
	ActivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	// DeactivateApi disables the API for the user with the given id.
	DeactivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	// ChangePassword changes the password for the user with the given id.
	ChangePassword(ctx context.Context, id domain.UserIdentifier, oldPassword, newPassword string) (*domain.User, error)
	// GetUserPeers returns all peers for the given user.
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	// GetUserPeerStats returns all peer stats for the given user.
	GetUserPeerStats(ctx context.Context, id domain.UserIdentifier) ([]domain.PeerStatus, error)
	// GetUserInterfaces returns all interfaces for the given user.
	GetUserInterfaces(ctx context.Context, id domain.UserIdentifier) ([]domain.Interface, error)
	// BulkDelete deletes multiple users.
	BulkDelete(ctx context.Context, ids []domain.UserIdentifier) error
	// BulkUpdate modifies multiple users.
	BulkUpdate(ctx context.Context, ids []domain.UserIdentifier, updateFn func(*domain.User)) error
}

type UserEndpoint struct {
	cfg           *config.Config
	userService   UserService
	authenticator Authenticator
	validator     Validator
}

func NewUserEndpoint(
	cfg *config.Config,
	authenticator Authenticator,
	validator Validator,
	userService UserService,
) UserEndpoint {
	return UserEndpoint{
		cfg:           cfg,
		userService:   userService,
		authenticator: authenticator,
		validator:     validator,
	}
}

func (e UserEndpoint) GetName() string {
	return "UserEndpoint"
}

func (e UserEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/user")
	apiGroup.Use(e.authenticator.LoggedIn())

	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("GET /all", e.handleAllGet())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("GET /{id}", e.handleSingleGet())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("PUT /{id}", e.handleUpdatePut())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("DELETE /{id}", e.handleDelete())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /new", e.handleCreatePost())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("GET /{id}/peers", e.handlePeersGet())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("GET /{id}/stats", e.handleStatsGet())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("GET /{id}/interfaces", e.handleInterfacesGet())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("POST /{id}/api/enable", e.handleApiEnablePost())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("POST /{id}/api/disable", e.handleApiDisablePost())
	apiGroup.With(e.authenticator.UserIdMatch("id")).HandleFunc("POST /{id}/change-password",
		e.handleChangePasswordPost())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /bulk-delete", e.handleBulkDelete())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /bulk-enable", e.handleBulkEnable())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /bulk-disable", e.handleBulkDisable())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /bulk-lock", e.handleBulkLock())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /bulk-unlock", e.handleBulkUnlock())
}

// handleAllGet returns a gorm Handler function.
//
// @ID users_handleAllGet
// @Tags Users
// @Summary Get all user records.
// @Produce json
// @Success 200 {object} []model.User
// @Failure 500 {object} model.Error
// @Router /user/all [get]
func (e UserEndpoint) handleAllGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := e.userService.GetAllUsers(r.Context())
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewUsers(users))
	}
}

// handleSingleGet returns a gorm Handler function.
//
// @ID users_handleSingleGet
// @Tags Users
// @Summary Get a single user record.
// @Produce json
// @Param id path string true "The user identifier"
// @Success 200 {object} model.User
// @Failure 500 {object} model.Error
// @Router /user/{id} [get]
func (e UserEndpoint) handleSingleGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		user, err := e.userService.GetUser(r.Context(), domain.UserIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewUser(user, true))
	}
}

// handleUpdatePut returns a gorm Handler function.
//
// @ID users_handleUpdatePut
// @Tags Users
// @Summary Update the user record.
// @Produce json
// @Param id path string true "The user identifier"
// @Param request body model.User true "The user data"
// @Success 200 {object} model.User
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id} [put]
func (e UserEndpoint) handleUpdatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		var user model.User
		if err := request.BodyJson(r, &user); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(user); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != user.Identifier {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "user id mismatch"})
			return
		}

		updateUser, err := e.userService.UpdateUser(r.Context(), model.NewDomainUser(&user))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewUser(updateUser, false))
	}
}

// handleCreatePost returns a gorm Handler function.
//
// @ID users_handleCreatePost
// @Tags Users
// @Summary Create the new user record.
// @Produce json
// @Param request body model.User true "The user data"
// @Success 200 {object} model.User
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/new [post]
func (e UserEndpoint) handleCreatePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user model.User
		if err := request.BodyJson(r, &user); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(user); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newUser, err := e.userService.CreateUser(r.Context(), model.NewDomainUser(&user))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewUser(newUser, false))
	}
}

// handlePeersGet returns a gorm Handler function.
//
// @ID users_handlePeersGet
// @Tags Users
// @Summary Get peers for the given user.
// @Param id path string true "The user identifier"
// @Produce json
// @Success 200 {object} []model.Peer
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id}/peers [get]
func (e UserEndpoint) handlePeersGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId := Base64UrlDecode(request.Path(r, "id"))
		if userId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		peers, err := e.userService.GetUserPeers(r.Context(), domain.UserIdentifier(userId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeers(peers))
	}
}

// handleStatsGet returns a gorm Handler function.
//
// @ID users_handleStatsGet
// @Tags Users
// @Summary Get peer stats for the given user.
// @Param id path string true "The user identifier"
// @Produce json
// @Success 200 {object} model.PeerStats
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id}/stats [get]
func (e UserEndpoint) handleStatsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId := Base64UrlDecode(request.Path(r, "id"))
		if userId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		stats, err := e.userService.GetUserPeerStats(r.Context(), domain.UserIdentifier(userId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewPeerStats(e.cfg.Statistics.CollectPeerData, stats))
	}
}

// handleInterfacesGet returns a gorm Handler function.
//
// @ID users_handleInterfacesGet
// @Tags Users
// @Summary Get interfaces for the given user. Returns an empty list if self provisioning is disabled.
// @Param id path string true "The user identifier"
// @Produce json
// @Success 200 {object} []model.Interface
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id}/interfaces [get]
func (e UserEndpoint) handleInterfacesGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId := Base64UrlDecode(request.Path(r, "id"))
		if userId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		peers, err := e.userService.GetUserInterfaces(r.Context(), domain.UserIdentifier(userId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewInterfaces(peers, nil))
	}
}

// handleDelete returns a gorm Handler function.
//
// @ID users_handleDelete
// @Tags Users
// @Summary Delete the user record.
// @Produce json
// @Param id path string true "The user identifier"
// @Success 204 "No content if deletion was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id} [delete]
func (e UserEndpoint) handleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Base64UrlDecode(request.Path(r, "id"))
		if id == "" {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		err := e.userService.DeleteUser(r.Context(), domain.UserIdentifier(id))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleApiEnablePost returns a gorm Handler function.
//
// @ID users_handleApiEnablePost
// @Tags Users
// @Summary Enable the REST API for the given user.
// @Produce json
// @Success 200 {object} model.User
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id}/api/enable [post]
func (e UserEndpoint) handleApiEnablePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId := Base64UrlDecode(request.Path(r, "id"))
		if userId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		user, err := e.userService.ActivateApi(r.Context(), domain.UserIdentifier(userId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewUser(user, true))
	}
}

// handleApiDisablePost returns a gorm Handler function.
//
// @ID users_handleApiDisablePost
// @Tags Users
// @Summary Disable the REST API for the given user.
// @Produce json
// @Success 200 {object} model.User
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id}/api/disable [post]
func (e UserEndpoint) handleApiDisablePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId := Base64UrlDecode(request.Path(r, "id"))
		if userId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		user, err := e.userService.DeactivateApi(r.Context(), domain.UserIdentifier(userId))
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewUser(user, false))
	}
}

// handleChangePasswordPost returns a gorm Handler function.
//
// @ID users_handleChangePasswordPost
// @Tags Users
// @Summary Change the password for the given user.
// @Produce json
// @Success 200 {object} model.User
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id}/change-password [post]
func (e UserEndpoint) handleChangePasswordPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId := Base64UrlDecode(request.Path(r, "id"))
		if userId == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		var passwordData struct {
			OldPassword    string `json:"OldPassword"`
			Password       string `json:"Password"`
			PasswordRepeat string `json:"PasswordRepeat"`
		}
		if err := request.BodyJson(r, &passwordData); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if passwordData.OldPassword == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "old password missing"})
			return
		}

		if passwordData.Password == "" {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "new password missing"})
			return
		}

		if passwordData.OldPassword == passwordData.Password {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "password did not change"})
			return
		}

		if passwordData.Password != passwordData.PasswordRepeat {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "password mismatch"})
			return
		}

		user, err := e.userService.ChangePassword(r.Context(), domain.UserIdentifier(userId),
			passwordData.OldPassword, passwordData.Password)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewUser(user, false))
	}
}

// handleBulkDelete returns a gorm Handler function.
//
// @ID users_handleBulkDelete
// @Tags Users
// @Summary Bulk delete selected users.
// @Produce json
// @Param request body model.BulkPeerRequest true "A list of user identifiers to delete"
// @Success 204 "No content if deletion was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/bulk-delete [post]
func (e UserEndpoint) handleBulkDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.BulkUserRequest
		if err := request.BodyJson(r, &req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		ids := make([]domain.UserIdentifier, len(req.Identifiers))
		for i, id := range req.Identifiers {
			ids[i] = domain.UserIdentifier(id)
		}

		err := e.userService.BulkDelete(r.Context(), ids)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleBulkEnable returns a gorm Handler function.
//
// @ID users_handleBulkEnable
// @Tags Users
// @Summary Bulk enable selected users.
// @Produce json
// @Param request body model.BulkPeerRequest true "A list of user identifiers to enable"
// @Success 204 "No content if action was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/bulk-enable [post]
func (e UserEndpoint) handleBulkEnable() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.BulkUserRequest
		if err := request.BodyJson(r, &req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		ids := make([]domain.UserIdentifier, len(req.Identifiers))
		for i, id := range req.Identifiers {
			ids[i] = domain.UserIdentifier(id)
		}

		err := e.userService.BulkUpdate(r.Context(), ids, func(user *domain.User) {
			user.Disabled = nil
		})
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleBulkDisable returns a gorm Handler function.
//
// @ID users_handleBulkDisable
// @Tags Users
// @Summary Bulk disable selected users.
// @Produce json
// @Param request body model.BulkPeerRequest true "A list of user identifiers to disable"
// @Success 204 "No content if action was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/bulk-disable [post]
func (e UserEndpoint) handleBulkDisable() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.BulkUserRequest
		if err := request.BodyJson(r, &req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		ids := make([]domain.UserIdentifier, len(req.Identifiers))
		for i, id := range req.Identifiers {
			ids[i] = domain.UserIdentifier(id)
		}

		now := time.Now()
		err := e.userService.BulkUpdate(r.Context(), ids, func(user *domain.User) {
			user.Disabled = &now
			user.DisabledReason = domain.DisabledReasonAdmin
		})
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleBulkLock returns a gorm Handler function.
//
// @ID users_handleBulkLock
// @Tags Users
// @Summary Bulk lock selected users.
// @Produce json
// @Param request body model.BulkPeerRequest true "A list of user identifiers to lock"
// @Success 204 "No content if action was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/bulk-lock [post]
func (e UserEndpoint) handleBulkLock() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.BulkUserRequest
		if err := request.BodyJson(r, &req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		ids := make([]domain.UserIdentifier, len(req.Identifiers))
		for i, id := range req.Identifiers {
			ids[i] = domain.UserIdentifier(id)
		}

		now := time.Now()
		err := e.userService.BulkUpdate(r.Context(), ids, func(user *domain.User) {
			user.Locked = &now
			user.LockedReason = domain.LockedReasonAdmin
		})
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}

// handleBulkUnlock returns a gorm Handler function.
//
// @ID users_handleBulkUnlock
// @Tags Users
// @Summary Bulk unlock selected users.
// @Produce json
// @Param request body model.BulkPeerRequest true "A list of user identifiers to unlock"
// @Success 204 "No content if action was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/bulk-unlock [post]
func (e UserEndpoint) handleBulkUnlock() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.BulkUserRequest
		if err := request.BodyJson(r, &req); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		ids := make([]domain.UserIdentifier, len(req.Identifiers))
		for i, id := range req.Identifiers {
			ids[i] = domain.UserIdentifier(id)
		}

		err := e.userService.BulkUpdate(r.Context(), ids, func(user *domain.User) {
			user.Locked = nil
		})
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}
