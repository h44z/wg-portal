package users

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// fakeInterfaceRepo records what the sync asked of the interface store.
type fakeInterfaceRepo struct {
	existing map[domain.InterfaceIdentifier]*domain.Interface
	getErr   error
	saved    map[domain.InterfaceIdentifier]*domain.Interface
}

func (f *fakeInterfaceRepo) GetInterface(
	_ context.Context,
	id domain.InterfaceIdentifier,
) (*domain.Interface, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if iface, ok := f.existing[id]; ok {
		return iface, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeInterfaceRepo) SaveInterface(
	_ context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(i *domain.Interface) (*domain.Interface, error),
) error {
	existing, ok := f.existing[id]
	if !ok {
		// Mirror the real repo, which creates the row when it is missing. The
		// point of the guard under test is that we never get here for an
		// interface that does not exist.
		existing = &domain.Interface{Identifier: id}
	}
	updated, err := updateFunc(existing)
	if err != nil {
		return err
	}
	if f.saved == nil {
		f.saved = make(map[domain.InterfaceIdentifier]*domain.Interface)
	}
	f.saved[id] = updated
	return nil
}

// recordingBus captures published events so the reconcile trigger can be
// asserted without wiring up the wireguard manager.
type recordingBus struct{ published []publishedEvent }

type publishedEvent struct {
	topic string
	args  []any
}

func (b *recordingBus) Publish(topic string, args ...any) {
	b.published = append(b.published, publishedEvent{topic: topic, args: args})
}

func newFilterTestManager(repo *fakeInterfaceRepo) Manager {
	return Manager{cfg: &config.Config{}, interfaces: repo, bus: &recordingBus{}}
}

func testProvider(revoke bool) *config.LdapProvider {
	return &config.LdapProvider{ProviderName: "ipa", RevokeOnFilterChange: revoke}
}

// An interface_filter entry naming an interface wg-portal does not know about
// must not bring that interface into existence. Creating it races the startup
// importer, which then fails with "interface already exists", and the row it
// leaves behind has no backend.
func TestApplyInterfaceLdapFilterDoesNotCreateInterfaces(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{}}
	m := newFilterTestManager(repo)

	m.applyInterfaceLdapFilter(context.Background(), "wg0", testProvider(false),
		[]domain.UserIdentifier{"alice"}, true)

	assert.Empty(t, repo.saved, "an unknown interface must not be created or written to")
}

func TestApplyInterfaceLdapFilterUpdatesExistingInterfaces(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{
		"wg0": {Identifier: "wg0", Backend: "opnsense1"},
	}}
	m := newFilterTestManager(repo)

	m.applyInterfaceLdapFilter(context.Background(), "wg0", testProvider(false),
		[]domain.UserIdentifier{"alice", "bob"}, true)

	require.Contains(t, repo.saved, domain.InterfaceIdentifier("wg0"))
	assert.Equal(t, []domain.UserIdentifier{"alice", "bob"},
		repo.saved["wg0"].LdapAllowedUsers["ipa"])
	assert.Equal(t, domain.InterfaceBackend("opnsense1"), repo.saved["wg0"].Backend,
		"the existing interface must be updated in place, not replaced")
}

// Several providers may filter the same interface; one must not clobber another.
func TestApplyInterfaceLdapFilterKeepsOtherProviders(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{
		"wg0": {
			Identifier: "wg0",
			LdapAllowedUsers: map[string][]domain.UserIdentifier{
				"other": {"carol"},
			},
		},
	}}
	m := newFilterTestManager(repo)

	m.applyInterfaceLdapFilter(context.Background(), "wg0", testProvider(false),
		[]domain.UserIdentifier{"alice"}, true)

	saved := repo.saved["wg0"].LdapAllowedUsers
	assert.Equal(t, []domain.UserIdentifier{"alice"}, saved["ipa"])
	assert.Equal(t, []domain.UserIdentifier{"carol"}, saved["other"],
		"another provider's allowed users must be left alone")
}

// A lookup failure that is not "not found" must also not fall through to a
// write, or a transient database error would silently create the interface.
func TestApplyInterfaceLdapFilterSkipsOnLookupError(t *testing.T) {
	repo := &fakeInterfaceRepo{
		existing: map[domain.InterfaceIdentifier]*domain.Interface{},
		getErr:   assert.AnError,
	}
	m := newFilterTestManager(repo)

	m.applyInterfaceLdapFilter(context.Background(), "wg0", testProvider(false),
		[]domain.UserIdentifier{"alice"}, true)

	assert.Empty(t, repo.saved, "a lookup error must not result in a write")
}

// With revoke_on_filter_change on, the resulting allowed set is published so a
// consumer can reconcile the interface's peers against it.
func TestApplyInterfaceLdapFilterPublishesEntitlement(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{
		"wg1": {Identifier: "wg1", LdapAllowedUsers: map[string][]domain.UserIdentifier{"ipa": {"alice", "bob"}}},
	}}
	m := newFilterTestManager(repo)
	bus := m.bus.(*recordingBus)

	m.applyInterfaceLdapFilter(context.Background(), "wg1", testProvider(true),
		[]domain.UserIdentifier{"alice"}, true)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "interface:ldapfilter:applied", bus.published[0].topic)
	assert.Equal(t, domain.InterfaceIdentifier("wg1"), bus.published[0].args[0])
	assert.Equal(t, []domain.UserIdentifier{"alice"}, bus.published[0].args[1],
		"the whole allowed set is published, not a diff, so reconciliation is idempotent")
}

// A tier whose last member leaves is not a broken filter: the directory is
// answering, it just matches nobody now. The empty set must still be published
// or the departing user keeps working access.
func TestApplyInterfaceLdapFilterPublishesEmptyTier(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{
		"wg1": {Identifier: "wg1", LdapAllowedUsers: map[string][]domain.UserIdentifier{"ipa": {"bob"}}},
	}}
	m := newFilterTestManager(repo)
	bus := m.bus.(*recordingBus)

	m.applyInterfaceLdapFilter(context.Background(), "wg1", testProvider(true), nil, true)

	require.Len(t, bus.published, 1, "an emptied tier must still be reconciled")
	assert.Empty(t, bus.published[0].args[1])
}

// Off by default, so upgrading does not suddenly start disabling peers.
func TestApplyInterfaceLdapFilterDoesNotPublishWhenDisabled(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{
		"wg1": {Identifier: "wg1", LdapAllowedUsers: map[string][]domain.UserIdentifier{"ipa": {"alice", "bob"}}},
	}}
	m := newFilterTestManager(repo)
	bus := m.bus.(*recordingBus)

	m.applyInterfaceLdapFilter(context.Background(), "wg1", testProvider(false),
		[]domain.UserIdentifier{"alice"}, true)

	assert.Empty(t, bus.published, "reconciliation is opt-in")
}

// If the directory returned no users at all the whole sync is suspect, so
// nothing is published and no peer is reconciled away.
func TestApplyInterfaceLdapFilterSilentWhenDirectoryReturnedNothing(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{
		"wg1": {Identifier: "wg1", LdapAllowedUsers: map[string][]domain.UserIdentifier{"ipa": {"alice", "bob"}}},
	}}
	m := newFilterTestManager(repo)
	bus := m.bus.(*recordingBus)

	m.applyInterfaceLdapFilter(context.Background(), "wg1", testProvider(true), nil, false)

	assert.Empty(t, bus.published,
		"an unhealthy directory must not revoke every peer on the interface")
}

// A field map naming an attribute the server does not return yields entries with
// no identifier. That tells the sync nothing about who is entitled, so it must
// not be mistaken for a directory that answered and happens to be empty --
// otherwise an empty filter revokes every owned peer on the interface.
func TestHasUsableIdentifiersIgnoresEntriesWithoutIdentifier(t *testing.T) {
	fields := makeTestLdapFields()

	withIds := []internal.RawLdapUser{{"uid": "alice"}, {"uid": "bob"}}
	assert.True(t, hasUsableIdentifiers(withIds, fields))

	// Entries came back, but the identifier attribute is absent from all of them.
	withoutIds := []internal.RawLdapUser{{"cn": "alice"}, {"cn": "bob"}}
	assert.False(t, hasUsableIdentifiers(withoutIds, fields))

	assert.False(t, hasUsableIdentifiers(nil, fields))
}
