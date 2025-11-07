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

type UserService interface {
	GetAll(ctx context.Context) ([]domain.User, error)
	GetById(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	Update(ctx context.Context, id domain.UserIdentifier, user *domain.User) (*domain.User, error)
	Delete(ctx context.Context, id domain.UserIdentifier) error
}

type UserEndpoint struct {
	users         UserService
	authenticator Authenticator
	validator     Validator
}

func NewUserEndpoint(
	authenticator Authenticator,
	validator Validator,
	userService UserService,
) *UserEndpoint {
	return &UserEndpoint{
		authenticator: authenticator,
		validator:     validator,
		users:         userService,
	}
}

func (e UserEndpoint) GetName() string {
	return "UserEndpoint"
}

func (e UserEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/user")
	apiGroup.Use(e.authenticator.LoggedIn())

	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("GET /all", e.handleAllGet())
	apiGroup.HandleFunc("GET /by-id/{id...}", e.handleByIdGet())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("POST /new", e.handleCreatePost())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("PUT /by-id/{id...}", e.handleUpdatePut())
	apiGroup.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("DELETE /by-id/{id...}", e.handleDelete())
}

// handleAllGet returns a gorm Handler function.
//
// @ID users_handleAllGet
// @Tags Users
// @Summary Get all user records.
// @Produce json
// @Success 200 {object} []models.User
// @Failure 401 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/all [get]
// @Security BasicAuth
func (e UserEndpoint) handleAllGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := e.users.GetAll(r.Context())
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewUsers(users))
	}
}

// handleByIdGet returns a gorm Handler function.
//
// @ID users_handleByIdGet
// @Tags Users
// @Summary Get a specific user record by its internal identifier.
// @Description Normal users can only access their own record. Admins can access all records.
// @Param id path string true "The user identifier."
// @Produce json
// @Success 200 {object} models.User
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/by-id/{id} [get]
// @Security BasicAuth
func (e UserEndpoint) handleByIdGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		user, err := e.users.GetById(r.Context(), domain.UserIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewUser(user, true))
	}
}

// handleCreatePost returns a gorm handler function.
//
// @ID users_handleCreatePost
// @Tags Users
// @Summary Create a new user record.
// @Description Only admins can create new records.
// @Param request body models.User true "The user data."
// @Produce json
// @Success 200 {object} models.User
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 409 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/new [post]
// @Security BasicAuth
func (e UserEndpoint) handleCreatePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user models.User
		if err := request.BodyJson(r, &user); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(user); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newUser, err := e.users.Create(r.Context(), models.NewDomainUser(&user))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewUser(newUser, true))
	}
}

// handleUpdatePut returns a gorm handler function.
//
// @ID users_handleUpdatePut
// @Tags Users
// @Summary Update a user record.
// @Description Only admins can update existing records.
// @Param id path string true "The user identifier."
// @Param request body models.User true "The user data."
// @Produce json
// @Success 200 {object} models.User
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/by-id/{id} [put]
// @Security BasicAuth
func (e UserEndpoint) handleUpdatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		var user models.User
		if err := request.BodyJson(r, &user); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validator.Struct(user); err != nil {
			respond.JSON(w, http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		updateUser, err := e.users.Update(r.Context(), domain.UserIdentifier(id), models.NewDomainUser(&user))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.JSON(w, http.StatusOK, models.NewUser(updateUser, true))
	}
}

// handleDelete returns a gorm handler function.
//
// @ID users_handleDelete
// @Tags Users
// @Summary Delete the user record.
// @Param id path string true "The user identifier."
// @Produce json
// @Success 204 "No content if deletion was successful."
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/by-id/{id} [delete]
// @Security BasicAuth
func (e UserEndpoint) handleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := request.Path(r, "id")
		if id == "" {
			respond.JSON(w, http.StatusBadRequest,
				models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		err := e.users.Delete(r.Context(), domain.UserIdentifier(id))
		if err != nil {
			status, model := ParseServiceError(err)
			respond.JSON(w, status, model)
			return
		}

		respond.Status(w, http.StatusNoContent)
	}
}
