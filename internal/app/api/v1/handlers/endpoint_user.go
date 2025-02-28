package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

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
	apiGroup.GET("/by-id/:id", authenticator.LoggedIn(), e.handleByIdGet())
	apiGroup.POST("/new", authenticator.LoggedIn(ScopeAdmin), e.handleCreatePost())
	apiGroup.PUT("/by-id/:id", authenticator.LoggedIn(ScopeAdmin), e.handleUpdatePut())
	apiGroup.DELETE("/by-id/:id", authenticator.LoggedIn(ScopeAdmin), e.handleDelete())
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
func (e UserEndpoint) handleAllGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		users, err := e.users.GetAll(ctx)
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
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/by-id/{id} [get]
// @Security BasicAuth
func (e UserEndpoint) handleByIdGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		user, err := e.users.GetById(ctx, domain.UserIdentifier(id))
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
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 409 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/new [post]
// @Security BasicAuth
func (e UserEndpoint) handleCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		var user models.User
		err := c.BindJSON(&user)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newUser, err := e.users.Create(ctx, models.NewDomainUser(&user))
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
func (e UserEndpoint) handleUpdatePut() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		var user models.User
		err := c.BindJSON(&user)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		updateUser, err := e.users.Update(ctx, domain.UserIdentifier(id), models.NewDomainUser(&user))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewUser(updateUser, true))
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
func (e UserEndpoint) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		err := e.users.Delete(ctx, domain.UserIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.Status(http.StatusNoContent)
	}
}
