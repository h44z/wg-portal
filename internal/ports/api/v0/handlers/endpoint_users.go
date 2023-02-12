package handlers

import (
	"net/http"

	"github.com/h44z/wg-portal/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/ports/api/v0/model"
)

type userEndpoint struct {
	app           *app.App
	authenticator *authenticationHandler
}

func (e userEndpoint) GetName() string {
	return "UserEndpoint"
}

func (e userEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/user", e.authenticator.LoggedIn())

	apiGroup.GET("/all", e.handleAllGet())
	apiGroup.PUT("/:id", e.handleUpdatePut())
	apiGroup.POST("/new", e.handleCreatePost())
	apiGroup.GET("/:id/peers", e.handlePeersGet())
}

// handleAllGet returns a gorm handler function.
//
// @ID users_handleAllGet
// @Tags Users
// @Summary Get all user records.
// @Produce json
// @Success 200 {object} []model.User
// @Failure 500 {object} model.Error
// @Router /users [get]
func (e userEndpoint) handleAllGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := e.app.Users.GetAll(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUsers(users))
	}
}

// handleUpdatePut returns a gorm handler function.
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
// @Router /users/{id} [put]
func (e userEndpoint) handleUpdatePut() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
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

		updateUser, err := e.app.Users.Update(ctx, model.NewDomainUser(&user))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUser(updateUser))
	}
}

// handleCreatePost returns a gorm handler function.
//
// @ID users_handleCreatePost
// @Tags Users
// @Summary Create the new user record.
// @Produce json
// @Param request body model.User true "The user data"
// @Success 200 {object} model.User
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /users/new [post]
func (e userEndpoint) handleCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		var user model.User
		err := c.BindJSON(&user)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newUser, err := e.app.Users.Create(ctx, model.NewDomainUser(&user))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUser(newUser))
	}
}

// handlePeersGet returns a gorm handler function.
//
// @ID users_handlePeersGet
// @Tags Users
// @Summary Get peers for the given user.
// @Produce json
// @Success 200 {object} []model.Peer
// @Failure 500 {object} model.Error
// @Router /users/{id}/peers [get]
func (e userEndpoint) handlePeersGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		interfaceId := c.Param("id")
		if interfaceId == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		peers, err := e.app.WireGuard.GetUserPeers(c.Request.Context(), domain.UserIdentifier(interfaceId))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewPeers(peers))
	}
}
