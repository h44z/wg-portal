package handlers

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/h44z/wg-portal/internal/app/api/v0/model"
)

type testEndpoint struct{}

func (e testEndpoint) GetName() string {
	return "TestEndpoint"
}

func (e testEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	g.GET("/now", e.handleCurrentTimeGet())
	g.GET("/hostname", e.handleHostnameGet())
}

// handleCurrentTimeGet represents the GET endpoint that responds the current time
//
// @ID test_handleCurrentTimeGet
// @Tags Testing
// @Summary Get the current local time.
// @Description Nothing more to describe...
// @Produce json
// @Success 200 {object} string
// @Failure 500 {object} model.Error
// @Router /now [get]
func (e testEndpoint) handleCurrentTimeGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		if time.Now().Second() == 0 {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code:    http.StatusInternalServerError,
				Message: "invalid time",
			})
		}
		c.JSON(http.StatusOK, time.Now().String())
	}
}

// handleHostnameGet represents the GET endpoint that responds the current hostname
//
// @ID test_handleHostnameGet
// @Tags Testing
// @Summary Get the current host name.
// @Description Nothing more to describe...
// @Produce json
// @Success 200 {object} string
// @Failure 500 {object} model.Error
// @Router /hostname [get]
func (e testEndpoint) handleHostnameGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		hostname, err := os.Hostname()
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
		}
		c.JSON(http.StatusOK, hostname)
	}
}
