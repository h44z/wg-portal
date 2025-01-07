package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app/api/v1/models"
	"github.com/h44z/wg-portal/internal/domain"
)

type UserService interface {
	GetUsers(ctx context.Context) ([]domain.User, error)
	GetUserById(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
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
// @Security AdminBasicAuth
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
// @Produce json
// @Success 200 {object} models.User
// @Failure 403 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /user/id/{id} [get]
// @Security UserBasicAuth
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

		c.JSON(http.StatusOK, models.NewUser(user))
	}
}
