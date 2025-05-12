package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal/app/api/core/request"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type AuthenticationService interface {
	// GetExternalLoginProviders returns a list of all available external login providers.
	GetExternalLoginProviders(_ context.Context) []domain.LoginProviderInfo
	// PlainLogin authenticates a user with a username and password.
	PlainLogin(ctx context.Context, username, password string) (*domain.User, error)
	// OauthLoginStep1 initiates the OAuth login flow.
	OauthLoginStep1(_ context.Context, providerId string) (authCodeUrl, state, nonce string, err error)
	// OauthLoginStep2 completes the OAuth login flow and logins the user in.
	OauthLoginStep2(ctx context.Context, providerId, nonce, code string) (*domain.User, error)
}

type WebAuthnService interface {
	Enabled() bool
	StartWebAuthnRegistration(ctx context.Context, userId domain.UserIdentifier) (
		responseOptions []byte,
		sessionData []byte,
		err error,
	)
	FinishWebAuthnRegistration(
		ctx context.Context,
		userId domain.UserIdentifier,
		name string,
		sessionDataAsJSON []byte,
		r *http.Request,
	) ([]domain.UserWebauthnCredential, error)
	GetCredentials(
		ctx context.Context,
		userId domain.UserIdentifier,
	) ([]domain.UserWebauthnCredential, error)
	RemoveCredential(
		ctx context.Context,
		userId domain.UserIdentifier,
		credentialIdBase64 string,
	) ([]domain.UserWebauthnCredential, error)
	UpdateCredential(
		ctx context.Context,
		userId domain.UserIdentifier,
		credentialIdBase64 string,
		name string,
	) ([]domain.UserWebauthnCredential, error)
	StartWebAuthnLogin(_ context.Context) (
		optionsAsJSON []byte,
		sessionDataAsJSON []byte,
		err error,
	)
	FinishWebAuthnLogin(
		ctx context.Context,
		sessionDataAsJSON []byte,
		r *http.Request,
	) (*domain.User, error)
}

type AuthEndpoint struct {
	cfg           *config.Config
	authService   AuthenticationService
	authenticator Authenticator
	session       Session
	validate      Validator
	webAuthn      WebAuthnService
}

func NewAuthEndpoint(
	cfg *config.Config,
	authenticator Authenticator,
	session Session,
	validator Validator,
	authService AuthenticationService,
	webAuthn WebAuthnService,
) AuthEndpoint {
	return AuthEndpoint{
		cfg:           cfg,
		authService:   authService,
		authenticator: authenticator,
		session:       session,
		validate:      validator,
		webAuthn:      webAuthn,
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

	apiGroup.HandleFunc("POST /webauthn/login/start", e.handleWebAuthnLoginStart())
	apiGroup.HandleFunc("POST /webauthn/login/finish", e.handleWebAuthnLoginFinish())
	apiGroup.With(e.authenticator.LoggedIn()).HandleFunc("GET /webauthn/credentials",
		e.handleWebAuthnCredentialsGet())
	apiGroup.With(e.authenticator.LoggedIn()).HandleFunc("POST /webauthn/register/start",
		e.handleWebAuthnRegisterStart())
	apiGroup.With(e.authenticator.LoggedIn()).HandleFunc("POST /webauthn/register/finish",
		e.handleWebAuthnRegisterFinish())
	apiGroup.With(e.authenticator.LoggedIn()).HandleFunc("DELETE /webauthn/credential/{id}",
		e.handleWebAuthnCredentialsDelete())
	apiGroup.With(e.authenticator.LoggedIn()).HandleFunc("PUT /webauthn/credential/{id}",
		e.handleWebAuthnCredentialsPut())

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
		providers := e.authService.GetExternalLoginProviders(r.Context())

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

		authCodeUrl, state, nonce, err := e.authService.OauthLoginStep1(context.Background(), provider)
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
		user, err := e.authService.OauthLoginStep2(loginCtx, provider, currentSession.OauthNonce,
			oauthCode)
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
	// start a fresh session
	e.session.DestroyData(r.Context())

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

		user, err := e.authService.PlainLogin(context.Background(), loginData.Username,
			loginData.Password)
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
// @ID auth_handleLogoutPost
// @Tags Authentication
// @Summary Get all available external login providers.
// @Produce json
// @Success 200 {object} model.Error
// @Router /auth/logout [post]
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
	if !strings.HasPrefix(returnUrl, e.cfg.Web.ExternalUrl) {
		return false
	}

	return true
}

// handleWebAuthnCredentialsGet returns a gorm Handler function.
//
// @ID auth_handleWebAuthnCredentialsGet
// @Tags Authentication
// @Summary Get all available external login providers.
// @Produce json
// @Success 200 {object} []model.WebAuthnCredentialResponse
// @Router /auth/webauthn/credentials [get]
func (e AuthEndpoint) handleWebAuthnCredentialsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !e.webAuthn.Enabled() {
			respond.JSON(w, http.StatusOK, []model.WebAuthnCredentialResponse{})
			return
		}

		currentSession := e.session.GetData(r.Context())

		userIdentifier := domain.UserIdentifier(currentSession.UserIdentifier)

		credentials, err := e.webAuthn.GetCredentials(r.Context(), userIdentifier)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewWebAuthnCredentialResponses(credentials))
	}
}

// handleWebAuthnCredentialsDelete returns a gorm Handler function.
//
// @ID auth_handleWebAuthnCredentialsDelete
// @Tags Authentication
// @Summary Delete a WebAuthn credential.
// @Param id path string true "Base64 encoded Credential ID"
// @Produce json
// @Success 200 {object} []model.WebAuthnCredentialResponse
// @Router /auth/webauthn/credential/{id} [delete]
func (e AuthEndpoint) handleWebAuthnCredentialsDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !e.webAuthn.Enabled() {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "WebAuthn is not enabled"})
			return
		}

		currentSession := e.session.GetData(r.Context())

		userIdentifier := domain.UserIdentifier(currentSession.UserIdentifier)

		credentialId := Base64UrlDecode(request.Path(r, "id"))

		credentials, err := e.webAuthn.RemoveCredential(r.Context(), userIdentifier, credentialId)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewWebAuthnCredentialResponses(credentials))
	}
}

// handleWebAuthnCredentialsPut returns a gorm Handler function.
//
// @ID auth_handleWebAuthnCredentialsPut
// @Tags Authentication
// @Summary Update a WebAuthn credential.
// @Param id path string true "Base64 encoded Credential ID"
// @Param request body model.WebAuthnCredentialRequest true "Credential name"
// @Produce json
// @Success 200 {object} []model.WebAuthnCredentialResponse
// @Router /auth/webauthn/credential/{id} [put]
func (e AuthEndpoint) handleWebAuthnCredentialsPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !e.webAuthn.Enabled() {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "WebAuthn is not enabled"})
			return
		}

		currentSession := e.session.GetData(r.Context())

		userIdentifier := domain.UserIdentifier(currentSession.UserIdentifier)

		credentialId := Base64UrlDecode(request.Path(r, "id"))
		var req model.WebAuthnCredentialRequest
		if err := request.BodyJson(r, &req); err != nil {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		credentials, err := e.webAuthn.UpdateCredential(r.Context(), userIdentifier, credentialId, req.Name)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewWebAuthnCredentialResponses(credentials))
	}
}

func (e AuthEndpoint) handleWebAuthnRegisterStart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !e.webAuthn.Enabled() {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "WebAuthn is not enabled"})
			return
		}

		currentSession := e.session.GetData(r.Context())

		userIdentifier := domain.UserIdentifier(currentSession.UserIdentifier)

		options, sessionData, err := e.webAuthn.StartWebAuthnRegistration(r.Context(), userIdentifier)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		currentSession.WebAuthnData = string(sessionData)
		e.session.SetData(r.Context(), currentSession)

		respond.Data(w, http.StatusOK, "application/json", options)
	}
}

// handleWebAuthnRegisterFinish returns a gorm Handler function.
//
// @ID auth_handleWebAuthnRegisterFinish
// @Tags Authentication
// @Summary Finish the WebAuthn registration process.
// @Param credential_name query string false "Credential name" default("")
// @Produce json
// @Success 200 {object} []model.WebAuthnCredentialResponse
// @Router /auth/webauthn/register/finish [post]
func (e AuthEndpoint) handleWebAuthnRegisterFinish() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !e.webAuthn.Enabled() {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "WebAuthn is not enabled"})
			return
		}

		name := request.QueryDefault(r, "credential_name", "")

		currentSession := e.session.GetData(r.Context())

		webAuthnSessionData := []byte(currentSession.WebAuthnData)
		currentSession.WebAuthnData = "" // clear the session data
		e.session.SetData(r.Context(), currentSession)

		credentials, err := e.webAuthn.FinishWebAuthnRegistration(
			r.Context(),
			domain.UserIdentifier(currentSession.UserIdentifier),
			name,
			webAuthnSessionData,
			r)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		respond.JSON(w, http.StatusOK, model.NewWebAuthnCredentialResponses(credentials))
	}
}

func (e AuthEndpoint) handleWebAuthnLoginStart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !e.webAuthn.Enabled() {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "WebAuthn is not enabled"})
			return
		}

		currentSession := e.session.GetData(r.Context())

		options, sessionData, err := e.webAuthn.StartWebAuthnLogin(r.Context())
		if err != nil {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		currentSession.WebAuthnData = string(sessionData)
		e.session.SetData(r.Context(), currentSession)

		respond.Data(w, http.StatusOK, "application/json", options)
	}
}

// handleWebAuthnLoginFinish returns a gorm Handler function.
//
// @ID auth_handleWebAuthnLoginFinish
// @Tags Authentication
// @Summary Finish the WebAuthn login process.
// @Produce json
// @Success 200 {object} model.User
// @Router /auth/webauthn/login/finish [post]
func (e AuthEndpoint) handleWebAuthnLoginFinish() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !e.webAuthn.Enabled() {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: "WebAuthn is not enabled"})
			return
		}

		currentSession := e.session.GetData(r.Context())

		webAuthnSessionData := []byte(currentSession.WebAuthnData)
		currentSession.WebAuthnData = "" // clear the session data
		e.session.SetData(r.Context(), currentSession)

		user, err := e.webAuthn.FinishWebAuthnLogin(
			r.Context(),
			webAuthnSessionData,
			r)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest,
				model.Error{Code: http.StatusBadRequest, Message: err.Error()})
			return
		}

		e.setAuthenticatedUser(r, user)

		respond.JSON(w, http.StatusOK, model.NewUser(user, false))
	}
}
