package handlers

import (
	model2 "github.com/h44z/wg-portal/internal/app/api/v0/model"
	"net/http"

	"github.com/h44z/wg-portal/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
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
		users, err := e.app.GetAllUsers(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model2.NewUsers(users))
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
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		var user model2.User
		err := c.BindJSON(&user)
		if err != nil {
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != user.Identifier {
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusBadRequest, Message: "user id mismatch"})
			return
		}

		updateUser, err := e.app.UpdateUser(ctx, model2.NewDomainUser(&user))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model2.NewUser(updateUser))
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

		var user model2.User
		err := c.BindJSON(&user)
		if err != nil {
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newUser, err := e.app.CreateUser(ctx, model2.NewDomainUser(&user))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model2.NewUser(newUser))
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
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		peers, err := e.app.GetUserPeers(c.Request.Context(), domain.UserIdentifier(interfaceId))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model2.NewPeers(peers))
	}
}
