package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
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

	apiGroup.GET("/all", e.authenticator.LoggedIn(ScopeAdmin), e.handleAllGet())
	apiGroup.GET("/:id", e.authenticator.UserIdMatch("id"), e.handleSingleGet())
	apiGroup.PUT("/:id", e.authenticator.UserIdMatch("id"), e.handleUpdatePut())
	apiGroup.DELETE("/:id", e.authenticator.UserIdMatch("id"), e.handleDelete())
	apiGroup.POST("/new", e.authenticator.LoggedIn(ScopeAdmin), e.handleCreatePost())
	apiGroup.GET("/:id/peers", e.authenticator.UserIdMatch("id"), e.handlePeersGet())
	apiGroup.GET("/:id/stats", e.authenticator.UserIdMatch("id"), e.handleStatsGet())
	apiGroup.GET("/:id/interfaces", e.authenticator.UserIdMatch("id"), e.handleInterfacesGet())
	apiGroup.POST("/:id/api/enable", e.authenticator.UserIdMatch("id"), e.handleApiEnablePost())
	apiGroup.POST("/:id/api/disable", e.authenticator.UserIdMatch("id"), e.handleApiDisablePost())
}

// handleAllGet returns a gorm handler function.
//
// @ID users_handleAllGet
// @Tags Users
// @Summary Get all user records.
// @Produce json
// @Success 200 {object} []model.User
// @Failure 500 {object} model.Error
// @Router /user/all [get]
func (e userEndpoint) handleAllGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		users, err := e.app.GetAllUsers(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUsers(users))
	}
}

// handleSingleGet returns a gorm handler function.
//
// @ID users_handleSingleGet
// @Tags Users
// @Summary Get a single user record.
// @Produce json
// @Param id path string true "The user identifier"
// @Success 200 {object} model.User
// @Failure 500 {object} model.Error
// @Router /user/{id} [get]
func (e userEndpoint) handleSingleGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing user id"})
			return
		}

		user, err := e.app.GetUser(ctx, domain.UserIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUser(user, true))
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
// @Router /user/{id} [put]
func (e userEndpoint) handleUpdatePut() gin.HandlerFunc {
	return func(c *gin.Context) {
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
// @Router /user/new [post]
func (e userEndpoint) handleCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		var user model.User
		err := c.BindJSON(&user)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newUser, err := e.app.CreateUser(ctx, model.NewDomainUser(&user))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUser(newUser, false))
	}
}

// handlePeersGet returns a gorm handler function.
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
func (e userEndpoint) handlePeersGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		userId := Base64UrlDecode(c.Param("id"))
		if userId == "" {
			c.JSON(http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		peers, err := e.app.GetUserPeers(ctx, domain.UserIdentifier(userId))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewPeers(peers))
	}
}

// handleStatsGet returns a gorm handler function.
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
func (e userEndpoint) handleStatsGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		userId := Base64UrlDecode(c.Param("id"))
		if userId == "" {
			c.JSON(http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		stats, err := e.app.GetUserPeerStats(ctx, domain.UserIdentifier(userId))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewPeerStats(e.app.Config.Statistics.CollectPeerData, stats))
	}
}

// handleInterfacesGet returns a gorm handler function.
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
func (e userEndpoint) handleInterfacesGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		userId := Base64UrlDecode(c.Param("id"))
		if userId == "" {
			c.JSON(http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		peers, err := e.app.GetUserInterfaces(ctx, domain.UserIdentifier(userId))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewInterfaces(peers, nil))
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
func (e userEndpoint) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
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

// handleApiEnablePost returns a gorm handler function.
//
// @ID users_handleApiEnablePost
// @Tags Users
// @Summary Enable the REST API for the given user.
// @Produce json
// @Success 200 {object} model.User
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id}/api/enable [post]
func (e userEndpoint) handleApiEnablePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		userId := Base64UrlDecode(c.Param("id"))
		if userId == "" {
			c.JSON(http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		user, err := e.app.ActivateApi(ctx, domain.UserIdentifier(userId))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUser(user, true))
	}
}

// handleApiDisablePost returns a gorm handler function.
//
// @ID users_handleApiDisablePost
// @Tags Users
// @Summary Disable the REST API for the given user.
// @Produce json
// @Success 200 {object} model.User
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /user/{id}/api/disable [post]
func (e userEndpoint) handleApiDisablePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		userId := Base64UrlDecode(c.Param("id"))
		if userId == "" {
			c.JSON(http.StatusBadRequest,
				model.Error{Code: http.StatusInternalServerError, Message: "missing id parameter"})
			return
		}

		user, err := e.app.DeactivateApi(ctx, domain.UserIdentifier(userId))
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, model.NewUser(user, false))
	}
}
