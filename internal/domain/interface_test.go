package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInterface_IsDisabledReturnsTrueWhenDisabled(t *testing.T) {
	iface := &Interface{}
	assert.False(t, iface.IsDisabled())

	now := time.Now()
	iface.Disabled = &now
	assert.True(t, iface.IsDisabled())
}

func TestInterface_AddressStrReturnsCorrectString(t *testing.T) {
	iface := &Interface{
		Addresses: []Cidr{
			{Cidr: "192.168.1.1/24", Addr: "192.168.1.1", NetLength: 24},
			{Cidr: "10.0.0.1/24", Addr: "10.0.0.1", NetLength: 24},
		},
	}
	expected := "192.168.1.1/24,10.0.0.1/24"
	assert.Equal(t, expected, iface.AddressStr())
}

func TestInterface_GetConfigFileNameReturnsCorrectFileName(t *testing.T) {
	iface := &Interface{Identifier: "wg0"}
	expected := "wg0.conf"
	assert.Equal(t, expected, iface.GetConfigFileName())

	iface.Identifier = "wg0@123"
	expected = "wg0123.conf"
	assert.Equal(t, expected, iface.GetConfigFileName())
}

func TestInterface_GetAllowedIPsReturnsCorrectCidrs(t *testing.T) {
	peer1 := Peer{
		Interface: PeerInterfaceConfig{
			Addresses: []Cidr{
				{Cidr: "192.168.1.2/32", Addr: "192.168.1.2", NetLength: 32},
			},
		},
	}
	peer2 := Peer{
		Interface: PeerInterfaceConfig{
			Addresses: []Cidr{
				{Cidr: "10.0.0.2/32", Addr: "10.0.0.2", NetLength: 32},
			},
		},
	}
	iface := &Interface{}
	expected := []Cidr{
		{Cidr: "192.168.1.2/32", Addr: "192.168.1.2", NetLength: 32},
		{Cidr: "10.0.0.2/32", Addr: "10.0.0.2", NetLength: 32},
	}
	assert.Equal(t, expected, iface.GetAllowedIPs([]Peer{peer1, peer2}))
}

func TestInterface_ManageRoutingTableReturnsCorrectValue(t *testing.T) {
	iface := &Interface{RoutingTable: "off"}
	assert.False(t, iface.ManageRoutingTable())

	iface.RoutingTable = "100"
	assert.True(t, iface.ManageRoutingTable())
}

func TestInterface_GetRoutingTableReturnsCorrectValue(t *testing.T) {
	iface := &Interface{RoutingTable: ""}
	assert.Equal(t, 0, iface.GetRoutingTable())

	iface.RoutingTable = "off"
	assert.Equal(t, -1, iface.GetRoutingTable())

	iface.RoutingTable = "0x64"
	assert.Equal(t, 100, iface.GetRoutingTable())

	iface.RoutingTable = "200"
	assert.Equal(t, 200, iface.GetRoutingTable())
}
