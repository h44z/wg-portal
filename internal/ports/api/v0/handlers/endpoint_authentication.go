package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/h44z/wg-portal/internal/domain"

	"github.com/gin-gonic/gin"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/ports/api/v0/model"
)

type authEndpoint struct {
	app           *app.App
	authenticator *authenticationHandler
}

func (e authEndpoint) GetName() string {
	return "AuthEndpoint"
}

func (e authEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/auth")

	apiGroup.GET("/providers", e.handleExternalLoginProvidersGet())
	apiGroup.GET("/session", e.handleSessionInfoGet())

	apiGroup.GET("/login/:provider/init", e.handleOauthInitiateGet())
	apiGroup.GET("/login/:provider/callback", e.handleOauthCallbackGet())

	apiGroup.POST("/login", e.handleLoginPost())
	apiGroup.POST("/logout", authenticator.LoggedIn(), e.handleLogoutPost())
}

// handleExternalLoginProvidersGet returns a gorm handler function.
//
// @ID auth_handleExternalLoginProvidersGet
// @Tags Authentication
// @Summary Get all available external login providers.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/providers [get]
func (e authEndpoint) handleExternalLoginProvidersGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		providers := e.app.Authenticator.GetExternalLoginProviders(c.Request.Context())

		c.JSON(http.StatusOK, model.NewLoginProviderInfos(providers))
	}
}

// handleSessionInfoGet returns a gorm handler function.
//
// @ID auth_handleSessionInfoGet
// @Tags Authentication
// @Summary Get information about the currently logged-in user.
// @Produce json
// @Success 200 {object} []model.SessionInfo
// @Failure 500 {object} model.Error
// @Router /auth/session [get]
func (e authEndpoint) handleSessionInfoGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := e.authenticator.Session.GetData(c)

		var loggedInUid *string
		var firstname *string
		var lastname *string
		var email *string

		if currentSession.LoggedIn {
			uid := string(currentSession.UserIdentifier)
			f := currentSession.Firstname
			l := currentSession.Lastname
			e := currentSession.Email
			loggedInUid = &uid
			firstname = &f
			lastname = &l
			email = &e
		}

		c.JSON(http.StatusOK, model.SessionInfo{
			LoggedIn:       currentSession.LoggedIn,
			IsAdmin:        currentSession.IsAdmin,
			UserIdentifier: loggedInUid,
			UserFirstname:  firstname,
			UserLastname:   lastname,
			UserEmail:      email,
		})
	}
}

// handleOauthInitiateGet returns a gorm handler function.
//
// @ID auth_handleOauthInitiateGet
// @Tags Authentication
// @Summary Initiate the OAuth login flow.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/{provider}/init [get]
func (e authEndpoint) handleOauthInitiateGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := e.authenticator.Session.GetData(c)

		autoRedirect, _ := strconv.ParseBool(c.DefaultQuery("redirect", "false"))
		returnTo := c.Query("return")
		provider := c.Param("provider")

		var returnUrl *url.URL
		var returnParams string
		redirectToReturn := func() {
			c.Redirect(http.StatusFound, returnUrl.String()+"?"+returnParams)
		}

		if returnTo != "" {
			if u, err := url.Parse(returnTo); err == nil {
				returnUrl = u
			}
			queryParams := returnUrl.Query()
			queryParams.Set("wgLoginState", "err") // by default, we set the state to error
			returnUrl.RawQuery = ""                // remove potential query params
			returnParams = queryParams.Encode()
		}

		if currentSession.LoggedIn {
			if autoRedirect {
				queryParams := returnUrl.Query()
				queryParams.Set("wgLoginState", "success")
				returnParams = queryParams.Encode()
				redirectToReturn()
			} else {
				c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "already logged in"})
			}
			return
		}

		authCodeUrl, state, nonce, err := e.app.Authenticator.OauthLoginStep1(c.Request.Context(), provider)
		if err != nil {
			if autoRedirect {
				redirectToReturn()
			} else {
				c.JSON(http.StatusInternalServerError, model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			}
			return
		}

		authSession := e.authenticator.Session.DefaultSessionData()
		authSession.OauthState = state
		authSession.OauthNonce = nonce
		authSession.OauthProvider = provider
		authSession.OauthReturnTo = returnTo
		e.authenticator.Session.SetData(c, authSession)

		if autoRedirect {
			c.Redirect(http.StatusFound, authCodeUrl)
		} else {
			c.JSON(http.StatusOK, model.OauthInitiationResponse{
				RedirectUrl: authCodeUrl,
				State:       state,
			})
		}
	}
}

// handleOauthCallbackGet returns a gorm handler function.
//
// @ID auth_handleOauthCallbackGet
// @Tags Authentication
// @Summary Handle the OAuth callback.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/{provider}/callback [get]
func (e authEndpoint) handleOauthCallbackGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := e.authenticator.Session.GetData(c)

		var returnUrl *url.URL
		var returnParams string
		redirectToReturn := func() {
			c.Redirect(http.StatusFound, returnUrl.String()+"?"+returnParams)
		}

		if currentSession.OauthReturnTo != "" {
			if u, err := url.Parse(currentSession.OauthReturnTo); err == nil {
				returnUrl = u
			}
			queryParams := returnUrl.Query()
			queryParams.Set("wgLoginState", "err") // by default, we set the state to error
			returnUrl.RawQuery = ""                // remove potential query params
			returnParams = queryParams.Encode()
		}

		if currentSession.LoggedIn {
			if returnUrl != nil {
				queryParams := returnUrl.Query()
				queryParams.Set("wgLoginState", "success")
				returnParams = queryParams.Encode()
				redirectToReturn()
			} else {
				c.JSON(http.StatusBadRequest, model.Error{Message: "already logged in"})
			}
			return
		}

		provider := c.Param("provider")
		oauthCode := c.Query("code")
		oauthState := c.Query("state")

		if provider != currentSession.OauthProvider {
			if returnUrl != nil {
				redirectToReturn()
			} else {
				c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "invalid oauth provider"})
			}
			return
		}
		if oauthState != currentSession.OauthState {
			if returnUrl != nil {
				redirectToReturn()
			} else {
				c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: "invalid oauth state"})
			}
			return
		}

		loginCtx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
		user, err := e.app.Authenticator.OauthLoginStep2(loginCtx, provider, currentSession.OauthNonce, oauthCode)
		cancel()
		if err != nil {
			if returnUrl != nil {
				redirectToReturn()
			} else {
				c.JSON(http.StatusUnauthorized, model.Error{Code: http.StatusUnauthorized, Message: err.Error()})
			}
			return
		}

		e.setAuthenticatedUser(c, user)

		if returnUrl != nil {
			queryParams := returnUrl.Query()
			queryParams.Set("wgLoginState", "success")
			returnParams = queryParams.Encode()
			redirectToReturn()
		} else {
			c.JSON(http.StatusOK, user)
		}
	}
}

func (e authEndpoint) setAuthenticatedUser(c *gin.Context, user *domain.User) {
	currentSession := e.authenticator.Session.GetData(c)

	currentSession.LoggedIn = true
	currentSession.IsAdmin = user.IsAdmin
	currentSession.UserIdentifier = string(user.Identifier)
	currentSession.Firstname = user.Firstname
	currentSession.Lastname = user.Lastname
	currentSession.Email = user.Email

	currentSession.OauthState = ""
	currentSession.OauthNonce = ""
	currentSession.OauthProvider = ""
	currentSession.OauthReturnTo = ""

	e.authenticator.Session.SetData(c, currentSession)
}

// handleLoginPost returns a gorm handler function.
//
// @ID auth_handleLoginPost
// @Tags Authentication
// @Summary Get all available external login providers.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/login [post]
func (e authEndpoint) handleLoginPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := e.authenticator.Session.GetData(c)
		if currentSession.LoggedIn {
			c.JSON(http.StatusOK, model.Error{Code: http.StatusOK, Message: "already logged in"})
			return
		}

		var loginData struct {
			Username string `json:"username" binding:"required,min=2"`
			Password string `json:"password" binding:"required,min=4"`
		}

		if err := c.ShouldBindJSON(&loginData); err != nil {
			c.JSON(http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		user, err := e.app.Authenticator.PlainLogin(c.Request.Context(), loginData.Username, loginData.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, model.Error{Code: http.StatusUnauthorized, Message: "login failed"})
			return
		}

		e.setAuthenticatedUser(c, user)

		c.JSON(http.StatusOK, user)
	}
}

// handleLogoutPost returns a gorm handler function.
//
// @ID auth_handleLogoutGet
// @Tags Authentication
// @Summary Get all available external login providers.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/logout [get]
func (e authEndpoint) handleLogoutPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := e.authenticator.Session.GetData(c)

		if !currentSession.LoggedIn { // Not logged in
			c.JSON(http.StatusOK, model.Error{Code: http.StatusOK, Message: "not logged in"})
			return
		}

		e.authenticator.Session.DestroyData(c)
		c.JSON(http.StatusOK, model.Error{Code: http.StatusOK, Message: "logout ok"})
	}
}
