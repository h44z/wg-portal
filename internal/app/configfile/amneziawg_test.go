package configfile

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

func TestAmneziaWGConfigTemplates(t *testing.T) {
	tpl, err := newTemplateHandler()
	require.NoError(t, err)
	options := domain.AmneziaWGOptions{"H1": "100-200", "Jc": "4", "I1": "<b 0x1234>"}
	iface := &domain.Interface{Identifier: "awg0", Backend: config.AmneziaWGBackendName, AmneziaWG: options, PrivateKey: "private"}
	reader, err := tpl.GetInterfaceConfig(iface, nil)
	require.NoError(t, err)
	contents, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(contents), "PrivateKey = private\nH1 = 100-200\nI1 = <b 0x1234>\nJc = 4\n")
	require.Contains(t, string(contents), "\nAddress = ")
	peer := &domain.Peer{AmneziaWG: options}
	peer.Interface.PrivateKey = "private"
	for _, style := range []string{domain.ConfigStyleWgQuick, domain.ConfigStyleRaw} {
		reader, err := tpl.GetPeerConfig(peer, style)
		require.NoError(t, err)
		contents, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Contains(t, string(contents), "PrivateKey = private\nH1 = 100-200\nI1 = <b 0x1234>\nJc = 4\n")
		require.True(t, strings.Index(string(contents), "H1 =") < strings.Index(string(contents), "[Peer]"))
	}
	peer.AmneziaWG = nil
	reader, err = tpl.GetPeerConfig(peer, domain.ConfigStyleWgQuick)
	require.NoError(t, err)
	contents, err = io.ReadAll(reader)
	require.NoError(t, err)
	require.NotContains(t, string(contents), "H1 =")
	require.Contains(t, string(contents), "PrivateKey = private\nAddress =")
}

type amneziaConfigRepo struct {
	iface domain.Interface
	peer  domain.Peer
}

func (r *amneziaConfigRepo) GetInterface(context.Context, domain.InterfaceIdentifier) (*domain.Interface, error) {
	return &r.iface, nil
}
func (r *amneziaConfigRepo) GetPeer(context.Context, domain.PeerIdentifier) (*domain.Peer, error) {
	peer := r.peer
	return &peer, nil
}
func (r *amneziaConfigRepo) GetInterfaceAndPeers(context.Context, domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error) {
	return &r.iface, []domain.Peer{r.peer}, nil
}

func TestAmneziaWGDownloadsAndQRUseParentOptions(t *testing.T) {
	tpl, err := newTemplateHandler()
	require.NoError(t, err)
	repo := &amneziaConfigRepo{
		iface: domain.Interface{Identifier: "awg0", Backend: config.AmneziaWGBackendName, AmneziaWG: domain.AmneziaWGOptions{"H1": "100-200", "Jc": "4"}},
		peer:  domain.Peer{Identifier: "peer", InterfaceIdentifier: "awg0"},
	}
	manager := Manager{wg: repo, tplHandler: tpl}
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	reader, err := manager.GetPeerConfig(ctx, "peer", domain.ConfigStyleWgQuick)
	require.NoError(t, err)
	contents, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(contents), "H1 = 100-200")
	reader, err = manager.GetPeerConfigQrCode(ctx, "peer", domain.ConfigStyleWgQuick)
	require.NoError(t, err)
	contents, err = io.ReadAll(reader)
	require.NoError(t, err)
	require.NotEmpty(t, contents)
	// Changing parent options affects the next download without editing peers.
	repo.iface.AmneziaWG["H1"] = "300-400"
	reader, err = manager.GetPeerConfig(ctx, "peer", domain.ConfigStyleWgQuick)
	require.NoError(t, err)
	contents, err = io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(contents), "H1 = 300-400")
}
