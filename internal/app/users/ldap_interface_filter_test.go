package users

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func newFilterTestManager(repo *fakeInterfaceRepo) Manager {
	return Manager{cfg: &config.Config{}, interfaces: repo}
}

// An interface_filter entry naming an interface wg-portal does not know about
// must not bring that interface into existence. Creating it races the startup
// importer, which then fails with "interface already exists", and the row it
// leaves behind has no backend.
func TestApplyInterfaceLdapFilterDoesNotCreateInterfaces(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{}}
	m := newFilterTestManager(repo)

	m.applyInterfaceLdapFilter(context.Background(), "wg0", "ipa",
		[]domain.UserIdentifier{"alice"})

	assert.Empty(t, repo.saved, "an unknown interface must not be created or written to")
}

func TestApplyInterfaceLdapFilterUpdatesExistingInterfaces(t *testing.T) {
	repo := &fakeInterfaceRepo{existing: map[domain.InterfaceIdentifier]*domain.Interface{
		"wg0": {Identifier: "wg0", Backend: "opnsense1"},
	}}
	m := newFilterTestManager(repo)

	m.applyInterfaceLdapFilter(context.Background(), "wg0", "ipa",
		[]domain.UserIdentifier{"alice", "bob"})

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

	m.applyInterfaceLdapFilter(context.Background(), "wg0", "ipa",
		[]domain.UserIdentifier{"alice"})

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

	m.applyInterfaceLdapFilter(context.Background(), "wg0", "ipa",
		[]domain.UserIdentifier{"alice"})

	assert.Empty(t, repo.saved, "a lookup error must not result in a write")
}
