package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/h44z/wg-portal/internal/config"
)

func TestPeer_IsDisabled(t *testing.T) {
	peer := &Peer{}
	assert.False(t, peer.IsDisabled())

	now := time.Now()
	peer.Disabled = &now
	assert.True(t, peer.IsDisabled())
}

func TestPeer_IsExpired(t *testing.T) {
	peer := &Peer{}
	assert.False(t, peer.IsExpired())

	expiredTime := time.Now().Add(-time.Hour)
	peer.ExpiresAt = &expiredTime
	assert.True(t, peer.IsExpired())

	futureTime := time.Now().Add(time.Hour)
	peer.ExpiresAt = &futureTime
	assert.False(t, peer.IsExpired())
}

func TestPeer_CheckAliveAddress(t *testing.T) {
	peer := &Peer{}
	assert.Equal(t, "", peer.CheckAliveAddress())

	peer.Interface.CheckAliveAddress = "192.168.1.1"
	assert.Equal(t, "192.168.1.1", peer.CheckAliveAddress())

	peer.Interface.CheckAliveAddress = ""
	peer.Interface.Addresses = []Cidr{{Addr: "10.0.0.1"}}
	assert.Equal(t, "10.0.0.1", peer.CheckAliveAddress())
}

func TestPeer_GetConfigFileName(t *testing.T) {
	peer := &Peer{DisplayName: "Test Peer"}
	expected := "Test_Peer.conf"
	assert.Equal(t, expected, peer.GetConfigFileName())

	peer.DisplayName = ""
	peer.Identifier = "12345678"
	expected = "wg_12345678.conf"
	assert.Equal(t, expected, peer.GetConfigFileName())
}

func TestPeer_ApplyInterfaceDefaults(t *testing.T) {
	peer := &Peer{
		Endpoint: ConfigOption[string]{
			Value:       "",
			Overridable: true,
		},
		EndpointPublicKey: ConfigOption[string]{
			Value:       "",
			Overridable: true,
		},
		AllowedIPsStr: ConfigOption[string]{
			Value:       "1.1.1.1/32",
			Overridable: false,
		},
	}
	iface := &Interface{
		PeerDefEndpoint: "192.168.1.1",
		KeyPair: KeyPair{
			PublicKey: "publicKey",
		},
		PeerDefAllowedIPsStr: "8.8.8.8/32",
	}

	peer.ApplyInterfaceDefaults(iface)
	assert.Equal(t, "192.168.1.1", peer.Endpoint.GetValue())
	assert.Equal(t, "publicKey", peer.EndpointPublicKey.GetValue())
	assert.Equal(t, "1.1.1.1/32", peer.AllowedIPsStr.GetValue())
}

func TestPeer_GenerateDisplayName(t *testing.T) {
	peer := &Peer{Identifier: "12345678"}
	peer.GenerateDisplayName("Prefix")
	expected := "Prefix Peer 12345678"
	assert.Equal(t, expected, peer.DisplayName)

	peer.GenerateDisplayName("")
	expected = "Peer 12345678"
	assert.Equal(t, expected, peer.DisplayName)
}

func TestPeer_OverwriteUserEditableFields(t *testing.T) {
	peer := &Peer{}
	userPeer := &Peer{
		DisplayName: "New DisplayName",
	}

	peer.OverwriteUserEditableFields(userPeer, &config.Config{})
	assert.Equal(t, "New DisplayName", peer.DisplayName)
}

func TestPeer_GetPresharedKey(t *testing.T) {
	physicalPeer := PhysicalPeer{}
	assert.Nil(t, physicalPeer.GetPresharedKey())

	physicalPeer.PresharedKey = "Q0evIJTOjhyy2o5J7whvrsvQC+FRL8A74vrw44YHUAk="
	key := physicalPeer.GetPresharedKey()
	assert.NotNil(t, key)
}

func TestPeer_GetEndpointAddress(t *testing.T) {
	physicalPeer := PhysicalPeer{}
	assert.Nil(t, physicalPeer.GetEndpointAddress())

	physicalPeer.Endpoint = "192.168.1.1:51820"
	addr := physicalPeer.GetEndpointAddress()
	assert.NotNil(t, addr)
	assert.Equal(t, "192.168.1.1:51820", addr.String())
}

func TestPeer_GetPersistentKeepaliveTime(t *testing.T) {
	physicalPeer := PhysicalPeer{}
	assert.Nil(t, physicalPeer.GetPersistentKeepaliveTime())

	physicalPeer.PersistentKeepalive = 25
	duration := physicalPeer.GetPersistentKeepaliveTime()
	assert.NotNil(t, duration)
	assert.Equal(t, 25*time.Second, *duration)
}

func TestPeer_GetAllowedIPs(t *testing.T) {
	physicalPeer := PhysicalPeer{}
	assert.Empty(t, physicalPeer.GetAllowedIPs())

	physicalPeer.AllowedIPs = []Cidr{
		{
			Cidr:      "192.168.1.0/24",
			Addr:      "192.168.1.0",
			NetLength: 24,
		},
	}
	ips := physicalPeer.GetAllowedIPs()
	assert.Len(t, ips, 1)
	assert.Equal(t, "192.168.1.0/24", ips[0].String())

	physicalPeer.AllowedIPs = []Cidr{
		{
			Cidr:      "192.168.1.0/24",
			Addr:      "192.168.1.0",
			NetLength: 24,
		},
		{
			Cidr:      "fe80::/64",
			Addr:      "fe80::",
			NetLength: 64,
		},
	}
	ips2 := physicalPeer.GetAllowedIPs()
	assert.Len(t, ips2, 2)
	assert.Equal(t, "192.168.1.0/24", ips2[0].String())
	assert.Equal(t, "fe80::/64", ips2[1].String())
}
