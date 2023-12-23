//go:build integration

package adapters

import (
	"context"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	MikrotikUrl  = "http://10.234.2.1/rest"
	MikrotikUser = "integtest"
	MikrotikPass = "SuperS3cret!"
)

func TestWgMikrotikRepo_GetInterfaces(t *testing.T) {
	w := NewWgMikrotikRepo(MikrotikUrl, MikrotikUser, MikrotikPass)
	got, err := w.GetInterfaces(context.Background())
	assert.NoError(t, err)
	assert.Equalf(t, 3, len(got), "GetInterfaces()")
}

func TestWgMikrotikRepo_GetInterface(t *testing.T) {
	w := NewWgMikrotikRepo(MikrotikUrl, MikrotikUser, MikrotikPass)
	got, err := w.GetInterface(context.Background(), "wgUser")
	assert.NoError(t, err)
	assert.Equalf(t, domain.InterfaceIdentifier("wgUser"), got.Identifier, "GetInterface()")
}

func TestWgMikrotikRepo_GetPeers(t *testing.T) {
	w := NewWgMikrotikRepo(MikrotikUrl, MikrotikUser, MikrotikPass)
	got, err := w.GetPeers(context.Background(), "wgUser")
	assert.NoError(t, err)
	assert.Equalf(t, 4, len(got), "GetPeers()")
}

func TestWgMikrotikRepo_GetPeer(t *testing.T) {
	w := NewWgMikrotikRepo(MikrotikUrl, MikrotikUser, MikrotikPass)
	got, err := w.GetPeer(context.Background(), "wgUser", "Ytfq6plqkOo95HAUYGrjiG3GU352NahLYLnE1cItDkI=")
	assert.NoError(t, err)
	assert.Equalf(t, domain.PeerIdentifier("Ytfq6plqkOo95HAUYGrjiG3GU352NahLYLnE1cItDkI="), got.Identifier, "GetPeer()")
}
