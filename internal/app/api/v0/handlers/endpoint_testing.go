package handlers

import (
	"net/http"
	"os"
	"time"

	"github.com/go-pkgz/routegroup"

	"github.com/fedor-git/wg-portal-2/internal/app/api/core/respond"
	"github.com/fedor-git/wg-portal-2/internal/app/api/v0/model"
)

type TestEndpoint struct {
	authenticator Authenticator
}

func NewTestEndpoint(authenticator Authenticator) TestEndpoint {
	return TestEndpoint{
		authenticator: authenticator,
	}
}

func (e TestEndpoint) GetName() string {
	return "TestEndpoint"
}

func (e TestEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	g.HandleFunc("GET /now", e.handleCurrentTimeGet())
	g.With(e.authenticator.LoggedIn(ScopeAdmin)).HandleFunc("GET /hostname", e.handleHostnameGet())
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
func (e TestEndpoint) handleCurrentTimeGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if time.Now().Second() == 0 {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code:    http.StatusInternalServerError,
				Message: "invalid time",
			})
		}
		respond.JSON(w, http.StatusOK, time.Now().String())
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
func (e TestEndpoint) handleHostnameGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname, err := os.Hostname()
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, model.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
		}
		respond.JSON(w, http.StatusOK, hostname)
	}
}
