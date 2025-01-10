package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app/api/v1/models"
	"github.com/h44z/wg-portal/internal/domain"
)

type InterfaceEndpointInterfaceService interface {
	GetAll(context.Context) ([]domain.Interface, [][]domain.Peer, error)
	GetById(context.Context, domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	Create(context.Context, *domain.Interface) (*domain.Interface, error)
	Update(context.Context, domain.InterfaceIdentifier, *domain.Interface) (*domain.Interface, []domain.Peer, error)
	Delete(context.Context, domain.InterfaceIdentifier) error
}

type InterfaceEndpoint struct {
	interfaces InterfaceEndpointInterfaceService
}

func NewInterfaceEndpoint(interfaceService InterfaceEndpointInterfaceService) *InterfaceEndpoint {
	return &InterfaceEndpoint{
		interfaces: interfaceService,
	}
}

func (e InterfaceEndpoint) GetName() string {
	return "InterfaceEndpoint"
}

func (e InterfaceEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/interface", authenticator.LoggedIn())

	apiGroup.GET("/all", authenticator.LoggedIn(ScopeAdmin), e.handleAllGet())
	apiGroup.GET("/by-id/:id", authenticator.LoggedIn(ScopeAdmin), e.handleByIdGet())

	apiGroup.POST("/new", authenticator.LoggedIn(ScopeAdmin), e.handleCreatePost())
	apiGroup.PUT("/by-id/:id", authenticator.LoggedIn(ScopeAdmin), e.handleUpdatePut())
	apiGroup.DELETE("/by-id/:id", authenticator.LoggedIn(ScopeAdmin), e.handleDelete())
}

// handleAllGet returns a gorm Handler function.
//
// @ID interface_handleAllGet
// @Tags Interfaces
// @Summary Get all interface records.
// @Produce json
// @Success 200 {object} []models.Interface
// @Failure 401 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/all [get]
// @Security BasicAuth
func (e InterfaceEndpoint) handleAllGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		allInterfaces, allPeersPerInterface, err := e.interfaces.GetAll(ctx)
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewInterfaces(allInterfaces, allPeersPerInterface))
	}
}

// handleByIdGet returns a gorm Handler function.
//
// @ID interfaces_handleByIdGet
// @Tags Interfaces
// @Summary Get a specific interface record by its identifier.
// @Param id path string true "The interface identifier."
// @Produce json
// @Success 200 {object} models.Interface
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/by-id/{id} [get]
// @Security BasicAuth
func (e InterfaceEndpoint) handleByIdGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		iface, interfacePeers, err := e.interfaces.GetById(ctx, domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewInterface(iface, interfacePeers))
	}
}

// handleCreatePost returns a gorm handler function.
//
// @ID interfaces_handleCreatePost
// @Tags Interfaces
// @Summary Create a new interface record.
// @Param request body models.Interface true "The interface data."
// @Produce json
// @Success 200 {object} models.Interface
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 409 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/new [post]
// @Security BasicAuth
func (e InterfaceEndpoint) handleCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		var iface models.Interface
		err := c.BindJSON(&iface)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		newInterface, err := e.interfaces.Create(ctx, models.NewDomainInterface(&iface))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewInterface(newInterface, nil))
	}
}

// handleUpdatePut returns a gorm handler function.
//
// @ID interfaces_handleUpdatePut
// @Tags Interfaces
// @Summary Update an interface record.
// @Param id path string true "The interface identifier."
// @Param request body models.Interface true "The interface data."
// @Produce json
// @Success 200 {object} models.Interface
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/by-id/{id} [put]
// @Security BasicAuth
func (e InterfaceEndpoint) handleUpdatePut() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		var iface models.Interface
		err := c.BindJSON(&iface)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		updatedInterface, updatedInterfacePeers, err := e.interfaces.Update(
			ctx,
			domain.InterfaceIdentifier(id),
			models.NewDomainInterface(&iface),
		)
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.JSON(http.StatusOK, models.NewInterface(updatedInterface, updatedInterfacePeers))
	}
}

// handleDelete returns a gorm handler function.
//
// @ID interfaces_handleDelete
// @Tags Interfaces
// @Summary Delete the interface record.
// @Param id path string true "The interface identifier."
// @Produce json
// @Success 204 "No content if deletion was successful."
// @Failure 400 {object} models.Error
// @Failure 401 {object} models.Error
// @Failure 403 {object} models.Error
// @Failure 404 {object} models.Error
// @Failure 500 {object} models.Error
// @Router /interface/by-id/{id} [delete]
// @Security BasicAuth
func (e InterfaceEndpoint) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.SetUserInfoFromGin(c)

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, models.Error{Code: http.StatusBadRequest, Message: "missing interface id"})
			return
		}

		err := e.interfaces.Delete(ctx, domain.InterfaceIdentifier(id))
		if err != nil {
			c.JSON(ParseServiceError(err))
			return
		}

		c.Status(http.StatusNoContent)
	}
}
