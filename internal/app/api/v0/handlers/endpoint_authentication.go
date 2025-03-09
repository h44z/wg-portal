package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/core/request"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
)

type Session interface {
	// SetData sets the session data for the given context.
	SetData(ctx context.Context, val SessionData)
	// GetData returns the session data for the given context. If no data is found, the default session data is returned.
	GetData(ctx context.Context) SessionData
	// DestroyData destroys the session data for the given context.
	DestroyData(ctx context.Context)
}

type Validator interface {
	Struct(s interface{}) error
}

type AuthEndpoint struct {
	app           *app.App
	authenticator Authenticator
	session       Session
	validate      Validator
}

func NewAuthEndpoint(app *app.App, authenticator Authenticator, session Session, validator Validator) AuthEndpoint {
	return AuthEndpoint{
		app:           app,
		authenticator: authenticator,
		session:       session,
		validate:      validator,
	}
}

func (e AuthEndpoint) GetName() string {
	return "AuthEndpoint"
}

func (e AuthEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/auth")

	apiGroup.HandleFunc("GET /providers", e.handleExternalLoginProvidersGet())
	apiGroup.HandleFunc("GET /session", e.handleSessionInfoGet())

	apiGroup.HandleFunc("GET /login/{provider}/init", e.handleOauthInitiateGet())
	apiGroup.HandleFunc("GET /login/{provider}/callback", e.handleOauthCallbackGet())

	apiGroup.HandleFunc("POST /login", e.handleLoginPost())
	apiGroup.With(e.authenticator.LoggedIn()).HandleFunc("POST /logout", e.handleLogoutPost())
}

// handleExternalLoginProvidersGet returns a gorm Handler function.
//
// @ID auth_handleExternalLoginProvidersGet
// @Tags Authentication
// @Summary Get all available external login providers.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/providers [get]
func (e AuthEndpoint) handleExternalLoginProvidersGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providers := e.app.Authenticator.GetExternalLoginProviders(r.Context())

		respond.JSON(w, http.StatusOK, model.NewLoginProviderInfos(providers))
	}
}

// handleSessionInfoGet returns a gorm Handler function.
//
// @ID auth_handleSessionInfoGet
// @Tags Authentication
// @Summary Get information about the currently logged-in user.
// @Produce json
// @Success 200 {object} []model.SessionInfo
// @Failure 500 {object} model.Error
// @Router /auth/session [get]
func (e AuthEndpoint) handleSessionInfoGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentSession := e.session.GetData(r.Context())

		var loggedInUid *string
		var firstname *string
		var lastname *string
		var email *string

		if currentSession.LoggedIn {
			uid := currentSession.UserIdentifier
			f := currentSession.Firstname
			l := currentSession.Lastname
			e := currentSession.Email
			loggedInUid = &uid
			firstname = &f
			lastname = &l
			email = &e
		}

		respond.JSON(w, http.StatusOK, model.SessionInfo{
			LoggedIn:       currentSession.LoggedIn,
			IsAdmin:        currentSession.IsAdmin,
			UserIdentifier: loggedInUid,
			UserFirstname:  firstname,
			UserLastname:   lastname,
			UserEmail:      email,
		})
	}
}

// handleOauthInitiateGet returns a gorm Handler function.
//
// @ID auth_handleOauthInitiateGet
// @Tags Authentication
// @Summary Initiate the OAuth login flow.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/{provider}/init [get]
func (e AuthEndpoint) handleOauthInitiateGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentSession := e.session.GetData(r.Context())

		autoRedirect, _ := strconv.ParseBool(request.QueryDefault(r, "redirect", "false"))
		returnTo := request.Query(r, "return")
		provider := request.Path(r, "provider")

		var returnUrl *url.URL
		var returnParams string
		redirectToReturn := func() {
			respond.Redirect(w, r, http.StatusFound, returnUrl.String()+"?"+returnParams)
		}

		if returnTo != "" {
			if !e.isValidReturnUrl(returnTo) {
				respond.JSON(w, http.StatusBadRequest,
					model.Error{Code: http.StatusBadRequest, Message: "invalid return URL"})
				return
			}
			if u, err := url.Parse(returnTo); err == nil {
				returnUrl = u
			}
			queryParams := returnUrl.Query()
			queryParams.Set("wgLoginState", "err") // by default, we set the state to error
			returnUrl.RawQuery = ""                // remove potential query params
			returnParams = queryParams.Encode()
		}

		if currentSession.LoggedIn {
			if autoRedirect && e.isValidReturnUrl(returnTo) {
				queryParams := returnUrl.Query()
				queryParams.Set("wgLoginState", "success")
				returnParams = queryParams.Encode()
				redirectToReturn()
			} else {
				respond.JSON(w, http.StatusBadRequest,
					model.Error{Code: http.StatusBadRequest, Message: "already logged in"})
			}
			return
		}

		authCodeUrl, state, nonce, err := e.app.Authenticator.OauthLoginStep1(context.Background(), provider)
		if err != nil {
			if autoRedirect && e.isValidReturnUrl(returnTo) {
				redirectToReturn()
			} else {
				respond.JSON(w, http.StatusInternalServerError,
					model.Error{Code: http.StatusInternalServerError, Message: err.Error()})
			}
			return
		}

		authSession := e.session.GetData(r.Context())
		authSession.OauthState = state
		authSession.OauthNonce = nonce
		authSession.OauthProvider = provider
		authSession.OauthReturnTo = returnTo
		e.session.SetData(r.Context(), authSession)

		if autoRedirect {
			respond.Redirect(w, r, http.StatusFound, authCodeUrl)
		} else {
			respond.JSON(w, http.StatusOK, model.OauthInitiationResponse{
				RedirectUrl: authCodeUrl,
				State:       state,
			})
		}
	}
}

// handleOauthCallbackGet returns a gorm Handler function.
//
// @ID auth_handleOauthCallbackGet
// @Tags Authentication
// @Summary Handle the OAuth callback.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/{provider}/callback [get]
func (e AuthEndpoint) handleOauthCallbackGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentSession := e.session.GetData(r.Context())

		var returnUrl *url.URL
		var returnParams string
		redirectToReturn := func() {
			respond.Redirect(w, r, http.StatusFound, returnUrl.String()+"?"+returnParams)
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
			if returnUrl != nil && e.isValidReturnUrl(returnUrl.String()) {
				queryParams := returnUrl.Query()
				queryParams.Set("wgLoginState", "success")
				returnParams = queryParams.Encode()
				redirectToReturn()
			} else {
				respond.JSON(w, http.StatusBadRequest, model.Error{Message: "already logged in"})
			}
			return
		}

		provider := request.Path(r, "provider")
		oauthCode := request.Query(r, "code")
		oauthState := request.Query(r, "state")

		if provider != currentSession.OauthProvider {
			if returnUrl != nil && e.isValidReturnUrl(returnUrl.String()) {
				redirectToReturn()
			} else {
				respond.JSON(w, http.StatusBadRequest,
					model.Error{Code: http.StatusBadRequest, Message: "invalid oauth provider"})
			}
			return
		}
		if oauthState != currentSession.OauthState {
			if returnUrl != nil && e.isValidReturnUrl(returnUrl.String()) {
				redirectToReturn()
			} else {
				respond.JSON(w, http.StatusBadRequest,
					model.Error{Code: http.StatusBadRequest, Message: "invalid oauth state"})
			}
			return
		}

		loginCtx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
		user, err := e.app.Authenticator.OauthLoginStep2(loginCtx, provider, currentSession.OauthNonce, oauthCode)
		cancel()
		if err != nil {
			if returnUrl != nil && e.isValidReturnUrl(returnUrl.String()) {
				redirectToReturn()
			} else {
				respond.JSON(w, http.StatusUnauthorized,
					model.Error{Code: http.StatusUnauthorized, Message: err.Error()})
			}
			return
		}

		e.setAuthenticatedUser(r, user)

		if returnUrl != nil && e.isValidReturnUrl(returnUrl.String()) {
			queryParams := returnUrl.Query()
			queryParams.Set("wgLoginState", "success")
			returnParams = queryParams.Encode()
			redirectToReturn()
		} else {
			respond.JSON(w, http.StatusOK, user)
		}
	}
}

func (e AuthEndpoint) setAuthenticatedUser(r *http.Request, user *domain.User) {
	currentSession := e.session.GetData(r.Context())

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

	e.session.SetData(r.Context(), currentSession)
}

// handleLoginPost returns a gorm Handler function.
//
// @ID auth_handleLoginPost
// @Tags Authentication
// @Summary Get all available external login providers.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/login [post]
func (e AuthEndpoint) handleLoginPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentSession := e.session.GetData(r.Context())
		if currentSession.LoggedIn {
			respond.JSON(w, http.StatusOK, model.Error{Code: http.StatusOK, Message: "already logged in"})
			return
		}

		var loginData struct {
			Username string `json:"username" binding:"required,min=2"`
			Password string `json:"password" binding:"required,min=4"`
		}

		if err := request.BodyJson(r, &loginData); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}
		if err := e.validate.Struct(loginData); err != nil {
			respond.JSON(w, http.StatusBadRequest, model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		user, err := e.app.Authenticator.PlainLogin(context.Background(), loginData.Username, loginData.Password)
		if err != nil {
			respond.JSON(w, http.StatusUnauthorized,
				model.Error{Code: http.StatusUnauthorized, Message: "login failed"})
			return
		}

		e.setAuthenticatedUser(r, user)

		respond.JSON(w, http.StatusOK, user)
	}
}

// handleLogoutPost returns a gorm Handler function.
//
// @ID auth_handleLogoutGet
// @Tags Authentication
// @Summary Get all available external login providers.
// @Produce json
// @Success 200 {object} []model.LoginProviderInfo
// @Router /auth/logout [get]
func (e AuthEndpoint) handleLogoutPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentSession := e.session.GetData(r.Context())

		if !currentSession.LoggedIn { // Not logged in
			respond.JSON(w, http.StatusOK, model.Error{Code: http.StatusOK, Message: "not logged in"})
			return
		}

		e.session.DestroyData(r.Context())
		respond.JSON(w, http.StatusOK, model.Error{Code: http.StatusOK, Message: "logout ok"})
	}
}

// isValidReturnUrl checks if the given return URL matches the configured external URL of the application.
func (e AuthEndpoint) isValidReturnUrl(returnUrl string) bool {
	if !strings.HasPrefix(returnUrl, e.app.Config.Web.ExternalUrl) {
		return false
	}

	return true
}
