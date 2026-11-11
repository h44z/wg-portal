package users

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// fakeUserRepo is a minimal UserDatabaseRepo that records which users were
// saved, so a test can assert whether the disable path ran at all.
type fakeUserRepo struct {
	users []domain.User
	saved map[domain.UserIdentifier]*domain.User
}

func (f *fakeUserRepo) GetAllUsers(_ context.Context) ([]domain.User, error) {
	return f.users, nil
}

func (f *fakeUserRepo) SaveUser(
	_ context.Context,
	id domain.UserIdentifier,
	updateFunc func(u *domain.User) (*domain.User, error),
) error {
	existing := &domain.User{Identifier: id}
	updated, err := updateFunc(existing)
	if err != nil {
		return err
	}
	if f.saved == nil {
		f.saved = make(map[domain.UserIdentifier]*domain.User)
	}
	f.saved[id] = updated
	return nil
}

func (f *fakeUserRepo) GetUser(_ context.Context, _ domain.UserIdentifier) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeUserRepo) GetUserByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeUserRepo) GetUserByWebAuthnCredential(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeUserRepo) FindUsers(_ context.Context, _ string) ([]domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) DeleteUser(_ context.Context, _ domain.UserIdentifier) error { return nil }

type fakeBus struct {
	published []string
}

func (f *fakeBus) Publish(topic string, _ ...any) {
	f.published = append(f.published, topic)
}

// ldapUser builds an existing wg-portal user that is sourced from LDAP and is
// therefore a candidate for being disabled when missing.
func ldapUser(id string) domain.User {
	return domain.User{
		Identifier: domain.UserIdentifier(id),
		Authentications: []domain.UserAuthentication{
			{Source: domain.UserSourceLdap, ProviderName: "testprovider"},
		},
	}
}

func newTestManager(repo *fakeUserRepo, bus *fakeBus) Manager {
	return Manager{cfg: &config.Config{}, bus: bus, users: repo}
}

// A directory that answers successfully but returns nothing usable must not be
// read as "every user was removed". Disabling every LDAP-sourced user at once
// takes down the VPN an admin would need in order to fix the directory.
func TestDisableMissingLdapUsers_RefusesOnEmptyDirectoryResult(t *testing.T) {
	repo := &fakeUserRepo{users: []domain.User{ldapUser("alice"), ldapUser("bob")}}
	bus := &fakeBus{}
	m := newTestManager(repo, bus)

	err := m.disableMissingLdapUsers(context.Background(), "testprovider",
		[]internal.RawLdapUser{}, makeTestLdapFields())

	require.NoError(t, err)
	assert.Empty(t, repo.saved, "no user may be disabled when LDAP returned no entries")
	assert.Empty(t, bus.published, "no disable events may be published")
}

// Same protection when the search returns entries but the configured identifier
// attribute is absent from all of them -- a misconfigured field map yields no
// usable identifiers, which would otherwise mark every user as missing.
func TestDisableMissingLdapUsers_RefusesWhenNoUsableIdentifiers(t *testing.T) {
	repo := &fakeUserRepo{users: []domain.User{ldapUser("alice"), ldapUser("bob")}}
	bus := &fakeBus{}
	m := newTestManager(repo, bus)

	// Entries exist, but none carry the "uid" attribute the field map expects.
	raw := []internal.RawLdapUser{
		{"dn": "cn=alice,dc=example,dc=com", "cn": "alice"},
		{"dn": "cn=bob,dc=example,dc=com", "cn": "bob"},
	}

	err := m.disableMissingLdapUsers(context.Background(), "testprovider", raw, makeTestLdapFields())

	require.NoError(t, err)
	assert.Empty(t, repo.saved, "no user may be disabled when no identifiers could be extracted")
	assert.Empty(t, bus.published)
}

// The guard must not suppress the feature: with a genuine directory result,
// users absent from it are still disabled.
func TestDisableMissingLdapUsers_DisablesGenuinelyMissingUsers(t *testing.T) {
	repo := &fakeUserRepo{users: []domain.User{ldapUser("alice"), ldapUser("bob")}}
	bus := &fakeBus{}
	m := newTestManager(repo, bus)

	// Only alice is still present in the directory.
	raw := []internal.RawLdapUser{{"dn": "uid=alice,dc=example,dc=com", "uid": "alice"}}

	err := m.disableMissingLdapUsers(context.Background(), "testprovider", raw, makeTestLdapFields())

	require.NoError(t, err)
	assert.NotContains(t, repo.saved, domain.UserIdentifier("alice"), "alice is present in LDAP")
	require.Contains(t, repo.saved, domain.UserIdentifier("bob"), "bob is missing and must be disabled")
	assert.Equal(t, domain.DisabledReasonLdapMissing, repo.saved["bob"].DisabledReason)
	assert.NotNil(t, repo.saved["bob"].Disabled)
	assert.Len(t, bus.published, 1)
}

// Users the operator has pinned with PersistLocalChanges, and users already
// disabled, must be left alone even when genuinely missing.
func TestDisableMissingLdapUsers_SkipsPinnedAndAlreadyDisabled(t *testing.T) {
	pinned := ldapUser("pinned")
	pinned.PersistLocalChanges = true

	alreadyDisabled := ldapUser("gone")
	disabledAt := time.Now()
	alreadyDisabled.Disabled = &disabledAt

	repo := &fakeUserRepo{users: []domain.User{pinned, alreadyDisabled}}
	bus := &fakeBus{}
	m := newTestManager(repo, bus)

	raw := []internal.RawLdapUser{{"dn": "uid=other,dc=example,dc=com", "uid": "other"}}

	err := m.disableMissingLdapUsers(context.Background(), "testprovider", raw, makeTestLdapFields())

	require.NoError(t, err)
	assert.Empty(t, repo.saved)
	assert.Empty(t, bus.published)
}
