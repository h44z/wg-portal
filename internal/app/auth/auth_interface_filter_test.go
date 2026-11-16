package auth

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/h44z/wg-portal/internal/domain"
)

type interfaceAccessCall struct {
	provider string
	userId   domain.UserIdentifier
	access   map[domain.InterfaceIdentifier]bool
}

type fakeInterfaceAccessUsers struct {
	UserManager
	user         *domain.User
	laterUser    *domain.User // returned from the second lookup on, e.g. a user locked during the login
	lookups      int
	caseIgnoring bool
	getErr       error
	registerErr  error
	onRegister   func()
	checkContext bool    // fail lookups and access updates on a done context, like the database does
	updateErrs   []error // returned by successive UpdateOauthInterfaceAccess calls
	calls        []interfaceAccessCall
	granted      map[domain.UserIdentifier]map[domain.InterfaceIdentifier]bool
}

func (f *fakeInterfaceAccessUsers) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.checkContext && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	f.lookups++
	user := f.user
	if f.laterUser != nil && f.lookups > 1 {
		user = f.laterUser
	}
	if user == nil || (user.Identifier != id && !f.caseIgnoring) {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (f *fakeInterfaceAccessUsers) UpdateUserInternal(_ context.Context, user *domain.User) (*domain.User, error) {
	return user, nil
}

func (f *fakeInterfaceAccessUsers) RegisterUser(_ context.Context, _ *domain.User) error {
	if f.onRegister != nil {
		f.onRegister()
	}
	return f.registerErr
}

func (f *fakeInterfaceAccessUsers) UpdateOauthInterfaceAccess(
	ctx context.Context,
	providerName string,
	userId domain.UserIdentifier,
	access map[domain.InterfaceIdentifier]bool,
) error {
	if f.checkContext && ctx.Err() != nil {
		return ctx.Err()
	}
	f.calls = append(f.calls, interfaceAccessCall{provider: providerName, userId: userId, access: access})
	if f.granted == nil {
		f.granted = make(map[domain.UserIdentifier]map[domain.InterfaceIdentifier]bool)
	}
	if f.granted[userId] == nil {
		f.granted[userId] = make(map[domain.InterfaceIdentifier]bool)
	}
	var err error
	if len(f.updateErrs) > 0 {
		err, f.updateErrs = f.updateErrs[0], f.updateErrs[1:]
	}
	for iface, allowed := range access {
		f.granted[userId][iface] = allowed
		if err != nil {
			break // simulate a failure part-way through
		}
	}
	return err
}

func (f *fakeInterfaceAccessUsers) hasGrant(userId domain.UserIdentifier) bool {
	for _, allowed := range f.granted[userId] {
		if allowed {
			return true
		}
	}
	return false
}

type fakeFilteredProvider struct {
	AuthenticatorOauth
	filter        map[string]*regexp.Regexp
	registration  bool
	userInfo      *domain.AuthenticatorUserInfo
	allowedGroups []string
}

func (f fakeFilteredProvider) RegistrationEnabled() bool { return f.registration }

func (f fakeFilteredProvider) GetName() string { return "oidc1" }

func (f fakeFilteredProvider) GetInterfaceFilter() map[string]*regexp.Regexp { return f.filter }

func (f fakeFilteredProvider) GetAllowedDomains() []string { return nil }

func (f fakeFilteredProvider) GetAllowedUserGroups() []string { return f.allowedGroups }

func (f fakeFilteredProvider) PKCETokenOptions(string) []oauth2.AuthCodeOption { return nil }

func (f fakeFilteredProvider) Exchange(context.Context, string, ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "token"}, nil
}

func (f fakeFilteredProvider) GetUserInfo(context.Context, *oauth2.Token, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (f fakeFilteredProvider) ParseUserInfo(map[string]any) (*domain.AuthenticatorUserInfo, error) {
	return f.userInfo, nil
}

type fakeEventBus struct{}

func (fakeEventBus) Publish(string, ...any) {}

func dataFilter() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{"wg-data": regexp.MustCompile("^vpn-data$")}
}

func TestUpdateInterfaceAccess(t *testing.T) {
	provider := fakeFilteredProvider{filter: map[string]*regexp.Regexp{
		"wg-data": regexp.MustCompile("^vpn-data$"),
		"wg-ops":  regexp.MustCompile("^(vpn-ops|sre)$"),
	}}

	tests := []struct {
		name   string
		groups []string
		expect map[domain.InterfaceIdentifier]bool
	}{
		{
			name:   "member of one group",
			groups: []string{"staff", "vpn-data"},
			expect: map[domain.InterfaceIdentifier]bool{"wg-data": true, "wg-ops": false},
		},
		{
			name:   "member of no group",
			groups: []string{"staff", "vpn-data-old"},
			expect: map[domain.InterfaceIdentifier]bool{"wg-data": false, "wg-ops": false},
		},
		{
			name:   "no groups claim",
			groups: nil,
			expect: map[domain.InterfaceIdentifier]bool{"wg-data": false, "wg-ops": false},
		},
		{
			name:   "member of both",
			groups: []string{"sre", "vpn-data"},
			expect: map[domain.InterfaceIdentifier]bool{"wg-data": true, "wg-ops": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeInterfaceAccessUsers{user: &domain.User{Identifier: "alice"}}
			a := &Authenticator{users: users}

			userId, err := a.updateInterfaceAccess(context.Background(), provider, "alice", tt.groups)
			require.NoError(t, err)
			assert.Equal(t, domain.UserIdentifier("alice"), userId)
			require.Len(t, users.calls, 1)
			assert.Equal(t, "oidc1", users.calls[0].provider)
			assert.Equal(t, domain.UserIdentifier("alice"), users.calls[0].userId)
			assert.Equal(t, tt.expect, users.calls[0].access)
		})
	}
}

func TestUpdateInterfaceAccess_NoFilter(t *testing.T) {
	users := &fakeInterfaceAccessUsers{}
	a := &Authenticator{users: users}

	userId, err := a.updateInterfaceAccess(context.Background(), fakeFilteredProvider{}, "alice",
		[]string{"vpn-data"})
	require.NoError(t, err)
	assert.Empty(t, userId)
	assert.Empty(t, users.calls, "providers without interface_filter must not touch interfaces")
}

func TestUpdateInterfaceAccess_PartialFailureIsRolledBack(t *testing.T) {
	users := &fakeInterfaceAccessUsers{
		user:       &domain.User{Identifier: "alice"},
		updateErrs: []error{assert.AnError},
	}
	a := &Authenticator{users: users}
	provider := fakeFilteredProvider{filter: map[string]*regexp.Regexp{
		"wg-data": regexp.MustCompile("^vpn-data$"),
		"wg-ops":  regexp.MustCompile("^vpn-data$"),
	}}

	_, err := a.updateInterfaceAccess(context.Background(), provider, "alice", []string{"vpn-data"})
	assert.ErrorIs(t, err, assert.AnError)
	assert.False(t, users.hasGrant("alice"), "a failed update must not leave a grant behind")
}

func TestUpdateInterfaceAccess_NewUserWithRegistration(t *testing.T) {
	users := &fakeInterfaceAccessUsers{}
	a := &Authenticator{users: users}
	provider := fakeFilteredProvider{filter: dataFilter(), registration: true}

	_, err := a.updateInterfaceAccess(context.Background(), provider, "alice", []string{"vpn-data"})
	require.NoError(t, err)
	require.Len(t, users.calls, 1)
	assert.Equal(t, map[domain.InterfaceIdentifier]bool{"wg-data": true}, users.calls[0].access)
}

func TestUpdateInterfaceAccess_UnknownUserWithoutRegistration(t *testing.T) {
	users := &fakeInterfaceAccessUsers{}
	a := &Authenticator{users: users}
	provider := fakeFilteredProvider{filter: dataFilter()}

	_, err := a.updateInterfaceAccess(context.Background(), provider, "alice", []string{"vpn-data"})
	require.NoError(t, err)
	assert.Empty(t, users.calls, "the login will fail, nothing must be recorded")
}

// Only "not found" means a new user; any other lookup error must not grant anything.
func TestUpdateInterfaceAccess_LookupErrorDoesNotGrant(t *testing.T) {
	users := &fakeInterfaceAccessUsers{getErr: assert.AnError}
	a := &Authenticator{users: users}
	provider := fakeFilteredProvider{filter: dataFilter(), registration: true}

	_, err := a.updateInterfaceAccess(context.Background(), provider, "alice", []string{"vpn-data"})
	assert.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, users.calls)
}

func TestUpdateInterfaceAccess_LockedUserOnlyRevokes(t *testing.T) {
	now := time.Now()
	users := &fakeInterfaceAccessUsers{user: &domain.User{Identifier: "alice", Locked: &now}}
	a := &Authenticator{users: users}
	provider := fakeFilteredProvider{filter: dataFilter()}

	_, err := a.updateInterfaceAccess(context.Background(), provider, "alice", []string{"vpn-data"})
	require.NoError(t, err)
	require.Len(t, users.calls, 1)
	assert.Equal(t, map[domain.InterfaceIdentifier]bool{"wg-data": false}, users.calls[0].access)
}

func TestUpdateInterfaceAccess_UsesStoredIdentifier(t *testing.T) {
	users := &fakeInterfaceAccessUsers{user: &domain.User{Identifier: "Alice"}, caseIgnoring: true}
	a := &Authenticator{users: users}
	provider := fakeFilteredProvider{filter: dataFilter()}

	userId, err := a.updateInterfaceAccess(context.Background(), provider, "alice", []string{"vpn-data"})
	require.NoError(t, err)
	assert.Equal(t, domain.UserIdentifier("Alice"), userId)
	require.Len(t, users.calls, 1)
	assert.Equal(t, domain.UserIdentifier("Alice"), users.calls[0].userId,
		"the grant must be keyed by the stored identifier, not the claim")
}

func TestUpdateInterfaceAccess_DisabledUserOnlyRevokes(t *testing.T) {
	now := time.Now()
	users := &fakeInterfaceAccessUsers{user: &domain.User{Identifier: "alice", Disabled: &now}}
	a := &Authenticator{users: users}

	_, err := a.updateInterfaceAccess(context.Background(), fakeFilteredProvider{filter: dataFilter()}, "alice",
		[]string{"vpn-data"})
	require.NoError(t, err)
	require.Len(t, users.calls, 1)
	assert.Equal(t, map[domain.InterfaceIdentifier]bool{"wg-data": false}, users.calls[0].access)
}

func newLoginTestAuthenticator(users UserManager, provider fakeFilteredProvider) *Authenticator {
	return &Authenticator{
		bus:                 fakeEventBus{},
		users:               users,
		oauthAuthenticators: map[string]AuthenticatorOauth{"oidc1": provider},
	}
}

// The provider authenticated the user, but the login is rejected: access granted earlier must be revoked.
func TestOauthLoginStep2_RejectedLoginRevokesGrant(t *testing.T) {
	for _, reason := range []string{"allowed_user_groups", "allowed_domains"} {
		t.Run(reason, func(t *testing.T) {
			users := &fakeInterfaceAccessUsers{user: &domain.User{Identifier: "Alice"}, caseIgnoring: true}
			users.granted = map[domain.UserIdentifier]map[domain.InterfaceIdentifier]bool{"Alice": {"wg-data": true}}
			provider := fakeFilteredProvider{
				filter:   dataFilter(),
				userInfo: &domain.AuthenticatorUserInfo{Identifier: "alice", Email: "alice@example.com"},
			}
			if reason == "allowed_user_groups" {
				provider.allowedGroups = []string{"staff"}
			}
			a := newLoginTestAuthenticator(users, provider)
			if reason == "allowed_domains" {
				a.oauthAuthenticators["oidc1"] = domainRestrictedProvider{provider}
			}

			_, _, err := a.OauthLoginStep2(context.Background(), "oidc1", "nonce", "code", "verifier")

			require.Error(t, err)
			assert.False(t, users.hasGrant("Alice"), "the stored access must be revoked")
		})
	}
}

type domainRestrictedProvider struct {
	fakeFilteredProvider
}

func (domainRestrictedProvider) GetAllowedDomains() []string { return []string{"example.org"} }

func TestOauthLoginStep2_RevokesGrantOfUserLockedDuringLogin(t *testing.T) {
	now := time.Now()
	users := &fakeInterfaceAccessUsers{
		user:      &domain.User{Identifier: "alice"},
		laterUser: &domain.User{Identifier: "alice", Locked: &now},
	}
	provider := fakeFilteredProvider{
		filter:   dataFilter(),
		userInfo: &domain.AuthenticatorUserInfo{Identifier: "alice", UserGroups: []string{"vpn-data"}},
	}

	_, _, err := newLoginTestAuthenticator(users, provider).OauthLoginStep2(context.Background(), "oidc1",
		"nonce", "code", "verifier")

	require.Error(t, err)
	require.NotEmpty(t, users.calls)
	assert.True(t, users.calls[0].access["wg-data"], "the first evaluation saw an unlocked user")
	assert.False(t, users.hasGrant("alice"), "the rejected login must not leave a grant behind")
}

// The login's context may expire while the user is registered; the revoke must still happen.
func TestOauthLoginStep2_RevokesGrantAfterContextExpired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	users := &fakeInterfaceAccessUsers{
		registerErr:  context.Canceled,
		onRegister:   cancel,
		checkContext: true,
	}
	provider := fakeFilteredProvider{
		filter:       dataFilter(),
		registration: true,
		userInfo:     &domain.AuthenticatorUserInfo{Identifier: "alice", UserGroups: []string{"vpn-data"}},
	}

	_, _, err := newLoginTestAuthenticator(users, provider).OauthLoginStep2(ctx, "oidc1", "nonce", "code", "verifier")

	require.Error(t, err)
	assert.False(t, users.hasGrant("alice"), "a failed login must not leave a grant behind")
}

func TestOauthLoginStep2_RevokesGrantWhenLoginFails(t *testing.T) {
	users := &fakeInterfaceAccessUsers{registerErr: assert.AnError}
	provider := fakeFilteredProvider{
		filter:       dataFilter(),
		registration: true,
		userInfo:     &domain.AuthenticatorUserInfo{Identifier: "alice", UserGroups: []string{"vpn-data"}},
	}
	a := &Authenticator{
		bus:                 fakeEventBus{},
		users:               users,
		oauthAuthenticators: map[string]AuthenticatorOauth{"oidc1": provider},
	}

	_, _, err := a.OauthLoginStep2(context.Background(), "oidc1", "nonce", "code", "verifier")
	require.Error(t, err)
	require.Len(t, users.calls, 2, "the grant must be followed by a compensating revoke")
	assert.True(t, users.calls[0].access["wg-data"])
	assert.False(t, users.hasGrant("alice"), "a failed login must not leave a grant behind")
}

func TestOauthLoginStep2_SuccessKeepsGrant(t *testing.T) {
	users := &fakeInterfaceAccessUsers{user: &domain.User{Identifier: "alice"}}
	provider := fakeFilteredProvider{
		filter:   dataFilter(),
		userInfo: &domain.AuthenticatorUserInfo{Identifier: "alice", UserGroups: []string{"vpn-data"}},
	}

	user, _, err := newLoginTestAuthenticator(users, provider).OauthLoginStep2(context.Background(), "oidc1",
		"nonce", "code", "verifier")

	require.NoError(t, err)
	assert.Equal(t, domain.UserIdentifier("alice"), user.Identifier)
	assert.Len(t, users.calls, 1, "a successful login must not revoke")
	assert.True(t, users.hasGrant("alice"))
}

func TestOauthLoginStep2_RejectedWhenStoringFails(t *testing.T) {
	users := &fakeInterfaceAccessUsers{
		user:       &domain.User{Identifier: "alice"},
		updateErrs: []error{assert.AnError},
	}
	provider := fakeFilteredProvider{
		filter:   dataFilter(),
		userInfo: &domain.AuthenticatorUserInfo{Identifier: "alice", UserGroups: []string{"vpn-data"}},
	}

	user, _, err := newLoginTestAuthenticator(users, provider).OauthLoginStep2(context.Background(), "oidc1",
		"nonce", "code", "verifier")

	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, user)
	assert.False(t, users.hasGrant("alice"))
}

func TestRevokeRejectedUserAccess_CancelledContext(t *testing.T) {
	users := &fakeInterfaceAccessUsers{user: &domain.User{Identifier: "alice"}, checkContext: true}
	users.granted = map[domain.UserIdentifier]map[domain.InterfaceIdentifier]bool{"alice": {"wg-data": true}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := &Authenticator{users: users}
	a.revokeRejectedUserAccess(ctx, fakeFilteredProvider{filter: dataFilter()}, "alice")

	assert.False(t, users.hasGrant("alice"))
}
