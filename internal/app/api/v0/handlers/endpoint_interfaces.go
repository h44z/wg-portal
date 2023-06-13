package handlers

import (
	model2 "github.com/h44z/wg-portal/internal/app/api/v0/model"
	"net/http"

	"github.com/h44z/wg-portal/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
)

type interfaceEndpoint struct {
	app           *app.App
	authenticator *authenticationHandler
}

func (e interfaceEndpoint) GetName() string {
	return "InterfaceEndpoint"
}

func (e interfaceEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/interface", e.authenticator.LoggedIn())

	apiGroup.GET("/prepare", e.handlePrepareGet())
	apiGroup.GET("/all", e.handleAllGet())
	apiGroup.GET("/get/:id", e.handleSingleGet())
	apiGroup.PUT("/:id", e.handleUpdatePut())
	apiGroup.POST("/new", e.handleCreatePost())

	apiGroup.GET("/peers/:id", e.handlePeersGet())
}

// handlePrepareGet returns a gorm handler function.
//
// @ID interfaces_handlePrepareGet
// @Tags Interface
// @Summary Prepare a new interface.
// @Produce json
// @Success 200 {object} model.Interface
// @Failure 500 {object} model.Error
// @Router /interface/prepare [get]
func (e interfaceEndpoint) handlePrepareGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		in, err := e.app.WireGuard.PrepareInterface(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model2.NewInterface(in))
	}
}

// handleAllGet returns a gorm handler function.
//
// @ID interfaces_handleAllGet
// @Tags Interface
// @Summary Get all available interfaces.
// @Produce json
// @Success 200 {object} []model.Interface
// @Failure 500 {object} model.Error
// @Router /interface/all [get]
func (e interfaceEndpoint) handleAllGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		interfaces, err := e.app.WireGuard.GetAllInterfaces(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model2.NewInterfaces(interfaces))
	}
}

// handleSingleGet returns a gorm handler function.
//
// @ID interfaces_handleSingleGet
// @Tags Interface
// @Summary Get single interface.
// @Produce json
// @Success 200 {object} model.Interface
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/get/{id} [get]
func (e interfaceEndpoint) handleSingleGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, model2.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		iface, _, err := e.app.WireGuard.GetInterfaceAndPeers(c.Request.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model2.NewInterface(iface))
	}
}

// handleUpdatePut returns a gorm handler function.
//
// @ID interfaces_handleUpdatePut
// @Tags Interface
// @Summary Update the interface record.
// @Produce json
// @Param id path string true "The interface identifier"
// @Param request body model.Interface true "The interface data"
// @Success 200 {object} model.Interface
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/{id} [put]
func (e interfaceEndpoint) handleUpdatePut() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		var in model2.Interface
		err := c.BindJSON(&in)
		if err != nil {
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != in.Identifier {
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusBadRequest, Message: "interface id mismatch"})
			return
		}

		updatedInterface, err := e.app.WireGuard.UpdateInterface(ctx, model2.NewDomainInterface(&in))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model2.NewInterface(updatedInterface))
	}
}

// handleCreatePost returns a gorm handler function.
//
// @ID interfaces_handleCreatePost
// @Tags Interface
// @Summary Create the new interface record.
// @Produce json
// @Param request body model.Interface true "The interface data"
// @Success 200 {object} model.Interface
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/new [post]
func (e interfaceEndpoint) handleCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		var in model2.Interface
		err := c.BindJSON(&in)
		if err != nil {
			c.JSON(http.StatusBadRequest, model2.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newInterface, err := e.app.WireGuard.CreateInterface(ctx, model2.NewDomainInterface(&in))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model2.NewInterface(newInterface))
	}
}

// handlePeersGet returns a gorm handler function.
//
// @ID interfaces_handlePeersGet
// @Tags Interface
// @Summary Get peers for the given interface.
// @Produce json
// @Success 200 {object} []model.Peer
// @Failure 500 {object} model.Error
// @Router /interface/peers/{id} [get]
func (e interfaceEndpoint) handlePeersGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, model2.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		_, peers, err := e.app.WireGuard.GetInterfaceAndPeers(c.Request.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model2.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model2.NewPeers(peers))
	}
}
