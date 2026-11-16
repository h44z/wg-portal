package users

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type hookBus struct {
	onPublish func(topic string, args ...any)
}

func (b *hookBus) Publish(topic string, args ...any) {
	b.onPublish(topic, args...)
}

// Default peers are created when the user-created event is handled, and only on interfaces the user is allowed
// to use. The LDAP sync must therefore store the interface_filter result before it creates users.
func TestSyncLdapUsers_UpdatesInterfaceFilterBeforeCreatingUsers(t *testing.T) {
	ifaces := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{
		"wg-data": {Identifier: "wg-data"},
	}}
	provider := &config.LdapProvider{
		ProviderName:       "ldap1",
		SyncFilter:         "(objectClass=person)",
		InterfaceFilter:    map[string]string{"wg-data": "(memberOf=cn=vpn,dc=example,dc=com)"},
		FieldMap:           *makeTestLdapFields(),
		ParsedAdminGroupDN: makeTestAdminGroupDN(t),
	}
	alice := internal.RawLdapUser{
		"dn":       "uid=alice,dc=example,dc=com",
		"uid":      "alice",
		"mail":     "alice@example.com",
		"memberOf": [][]byte{[]byte("cn=vpn,dc=example,dc=com")},
	}
	findUsers := func(filter string) ([]internal.RawLdapUser, error) {
		return []internal.RawLdapUser{alice}, nil // alice matches both the sync and the interface filter
	}

	var allowedOnCreation []domain.UserIdentifier
	created := false
	bus := &hookBus{onPublish: func(topic string, args ...any) {
		if topic != app.TopicUserCreated {
			return
		}
		created = true
		if saved, ok := ifaces.saved["wg-data"]; ok {
			allowedOnCreation = saved.LdapAllowedUsers["ldap1"]
		}
	}}
	m := Manager{cfg: &config.Config{}, bus: bus, users: &fakeUserRepo{}, interfaces: ifaces}

	require.NoError(t, m.syncLdapUsers(context.Background(), provider, findUsers))

	require.True(t, created, "the sync must create alice")
	assert.Equal(t, []domain.UserIdentifier{"alice"}, allowedOnCreation,
		"alice must already be allowed on wg-data when her creation is published")
}
