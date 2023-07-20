package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
	"io"
	"net/http"
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
	apiGroup.DELETE("/:id", e.handleDelete())
	apiGroup.POST("/new", e.handleCreatePost())
	apiGroup.GET("/config/:id", e.handleConfigGet())
	apiGroup.POST("/:id/save-config", e.handleSaveConfigPost())
	apiGroup.POST("/:id/apply-peer-defaults", e.handleApplyPeerDefaultsPost())

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
		in, err := e.app.PrepareInterface(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model.NewInterface(in))
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
		interfaces, err := e.app.GetAllInterfaces(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model.NewInterfaces(interfaces))
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
		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		iface, _, err := e.app.GetInterfaceAndPeers(c.Request.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model.NewInterface(iface))
	}
}

// handleConfigGet returns a gorm handler function.
//
// @ID interfaces_handleConfigGet
// @Tags Interface
// @Summary Get interface configuration as string.
// @Produce json
// @Success 200 {object} string
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/config/{id} [get]
func (e interfaceEndpoint) handleConfigGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		config, err := e.app.GetInterfaceConfig(c.Request.Context(), domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		configString, err := io.ReadAll(config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, string(configString))
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

		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		var in model.Interface
		err := c.BindJSON(&in)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != in.Identifier {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "interface id mismatch"})
			return
		}

		updatedInterface, err := e.app.UpdateInterface(ctx, model.NewDomainInterface(&in))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model.NewInterface(updatedInterface))
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

		var in model.Interface
		err := c.BindJSON(&in)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newInterface, err := e.app.CreateInterface(ctx, model.NewDomainInterface(&in))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model.NewInterface(newInterface))
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
		ctx := domain.SetUserInfoFromGin(c)

		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{
				Code: http.StatusInternalServerError, Message: "missing id parameter",
			})
			return
		}

		_, peers, err := e.app.GetInterfaceAndPeers(ctx, domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, model.NewPeers(peers))
	}
}

// handleDelete returns a gorm handler function.
//
// @ID interfaces_handleDelete
// @Tags Interface
// @Summary Delete the interface record.
// @Produce json
// @Param id path string true "The interface identifier"
// @Success 204 "No content if deletion was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/{id} [delete]
func (e interfaceEndpoint) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		err := e.app.DeleteInterface(ctx, domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// handleSaveConfigPost returns a gorm handler function.
//
// @ID interfaces_handleSaveConfigPost
// @Tags Interface
// @Summary Save the interface configuration in wg-quick format to a file.
// @Produce json
// @Param id path string true "The interface identifier"
// @Success 204 "No content if saving the configuration was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/{id}/save-config [post]
func (e interfaceEndpoint) handleSaveConfigPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		//ctx := domain.SetUserInfoFromGin(c)

		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		// TODO: implement

		c.Status(http.StatusNoContent)
	}
}

// handleApplyPeerDefaultsPost returns a gorm handler function.
//
// @ID interfaces_handleApplyPeerDefaultsPost
// @Tags Interface
// @Summary Apply all peer defaults to the available peers.
// @Produce json
// @Param id path string true "The interface identifier"
// @Param request body model.Interface true "The interface data"
// @Success 204 "No content if applying peer defaults was successful"
// @Failure 400 {object} model.Error
// @Failure 500 {object} model.Error
// @Router /interface/{id}/apply-peer-defaults [post]
func (e interfaceEndpoint) handleApplyPeerDefaultsPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := Base64UrlDecode(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		var in model.Interface
		err := c.BindJSON(&in)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		if id != in.Identifier {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "interface id mismatch"})
			return
		}

		err = e.app.ApplyPeerDefaults(ctx, model.NewDomainInterface(&in))
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code: http.StatusInternalServerError, Message: err.Error(),
			})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
