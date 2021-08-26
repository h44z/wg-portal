//go:build integration
// +build integration

// In Goland you can use File-Nesting to enhance the project view: _int_test.go; _test.go

// Run integrations tests as root!

package wireguard

import (
	"os/exec"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

func prepareTest(dev DeviceIdentifier) {
	_ = netlink.LinkDel(&netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: string(dev),
		},
		LinkType: "wireguard",
	})
}

func TestManagementUtil_CreateDevice(t *testing.T) {
	devName := DeviceIdentifier("wg666")
	prepareTest(devName)
	m := ManagementUtil{interfaces: make(map[DeviceIdentifier]InterfaceConfig), nl: NetlinkManager{}}

	defer m.DeleteDevice(devName)
	err := m.CreateDevice(devName)
	assert.NoError(t, err)

	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), devName)
}

func TestManagementUtil_DeleteDevice(t *testing.T) {
	devName := DeviceIdentifier("wg667")
	prepareTest(devName)
	m := ManagementUtil{interfaces: make(map[DeviceIdentifier]InterfaceConfig), nl: NetlinkManager{}}

	err := m.CreateDevice(devName)
	assert.NoError(t, err)
	err = m.DeleteDevice(devName)
	assert.NoError(t, err)

	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.NotContains(t, string(out), devName)
}

func TestManagementUtil_deviceExists(t *testing.T) {
	m := ManagementUtil{interfaces: make(map[DeviceIdentifier]InterfaceConfig)}
	assert.False(t, m.deviceExists("test"))

	m = ManagementUtil{interfaces: map[DeviceIdentifier]InterfaceConfig{"test": {}}}
	assert.True(t, m.deviceExists("test"))
}

func TestManagementUtil_UpdateDevice(t *testing.T) {
	devName := DeviceIdentifier("wg668")
	prepareTest(devName)
	wg, err := wgctrl.New()
	if !assert.NoError(t, err) {
		return
	}
	m := ManagementUtil{interfaces: make(map[DeviceIdentifier]InterfaceConfig), nl: NetlinkManager{}, wg: wg}

	defer m.DeleteDevice(devName)
	err = m.CreateDevice(devName)
	if !assert.NoError(t, err) {
		return
	}

	err = m.UpdateDevice(devName, InterfaceConfig{AddressStr: "123.123.123.123/24", Mtu: 1234})
	assert.NoError(t, err)

	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), "123.123.123.123")

	err = m.UpdateDevice(devName, InterfaceConfig{AddressStr: "123.123.123.123/24,fd9f:6666::10:6:6:1/64", Mtu: 1600})
	assert.NoError(t, err)

	cmd = exec.Command("ip", "addr")
	out, err = cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), "123.123.123.123")
	assert.Contains(t, string(out), "fd9f:6666::10:6:6:1")
}
