package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/app/api/v1/models"
	"github.com/h44z/wg-portal/internal/domain"
)

type UserService interface {
	GetUsers(ctx context.Context) ([]domain.User, error)
	GetUserById(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
}

type UserEndpoint struct {
	users UserService
}

func NewUserEndpoint(userService UserService) *UserEndpoint {
	return &UserEndpoint{
		users: userService,
	}
}

func (e UserEndpoint) GetName() string {
	return "UserEndpoint"
}

func (e UserEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/user", authenticator.LoggedIn())

	apiGroup.GET("/all", authenticator.LoggedIn(ScopeAdmin), e.handleAllGet())
	apiGroup.GET("/id/:id", authenticator.LoggedIn(), e.handleByIdGet())
}

// handleAllGet returns a gorm Handler function.
//
// @ID users_handleAllGet
// @Tags Users
// @Summary Get all user records.
// @Produce json
// @Success 200 {object} []models.User
// @Failure 500 {object} models.Error
// @Router /user/all [get]
// @Security BasicAuth
func (e UserEndpoint) handleAllGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		users, err := e.users.GetUsers(ctx)
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewUsers(users))
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
// @Failure 403 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/id/{id} [get]
// @Security BasicAuth
func (e UserEndpoint) handleByIdGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		user, err := e.users.GetUserById(ctx, domain.UserIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewUser(user, true))
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
// @Failure 403 {object} models.Error
// @Failure 409 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/new [post]
func (e UserEndpoint) handleCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		var user models.User
		err := c.BindJSON(&user)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newUser, err := e.users.CreateUser(ctx, models.NewDomainUser(&user))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewUser(newUser, true))
	}
}

// handleUpdatePut returns a gorm handler function.
//
// @ID users_handleUpdatePut
// @Tags Users
// @Summary Update a user record.
// @Description Only admins can update existing records.
// @Param id path string true "The user identifier"
// @Param request body models.User true "The user data"
// @Produce json
// @Success 200 {object} models.User
// @Failure 400 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/{id} [put]
func (e UserEndpoint) handleUpdatePut() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: implement
		ctx := domain.SetUserInfoFromGin(c)

		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		var user model.User
		err := c.BindJSON(&user)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != user.Identifier {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "user id mismatch"})
			return
		}

		updateUser, err := e.app.UpdateUser(ctx, model.NewDomainUser(&user))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUser(updateUser, false))
	}
}

// handleDelete returns a gorm handler function.
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
func (e UserEndpoint) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: implement
		ctx := domain.SetUserInfoFromGin(c)

		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		err := e.app.DeleteUser(ctx, domain.UserIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
