//go:build functional && linux
// +build functional,linux

// Run integrations tests as root!

package wireguard

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/h44z/wg-portal/internal/lowlevel"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// setup WireGuard manager with no linked store
func setup(t *testing.T) Manager {
	if getProcessOwner() != "root" {
		t.Fatalf("this tests need to be executed as root user")
	}

	wg, err := wgctrl.New()
	require.NoError(t, err)

	nl := &lowlevel.NetlinkManager{}

	manager, err := NewPersistentManager(wg, nl, nil) // No Store, all in memory
	require.NoError(t, err)

	return manager
}

func getProcessOwner() string {
	stdout, err := exec.Command("ps", "-o", "user=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return strings.TrimSpace(string(stdout))
}

func TestWireGuardCreateInterface(t *testing.T) {
	mgr := setup(t)

	interfaceName := persistence.InterfaceIdentifier("wg_test_001")
	defer mgr.DeleteInterface(interfaceName)

	err := mgr.CreateInterface(interfaceName)
	assert.NoError(t, err)

	// Validate that the interface has been created
	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), interfaceName)
}

func TestWireGuardDeleteInterface(t *testing.T) {
	mgr := setup(t)

	interfaceName := persistence.InterfaceIdentifier("wg_test_001")
	defer mgr.DeleteInterface(interfaceName)

	err := mgr.CreateInterface(interfaceName)
	assert.NoError(t, err)

	err = mgr.DeleteInterface(interfaceName)
	assert.NoError(t, err)

	// Validate that the interface has been deleted
	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.NotContains(t, string(out), interfaceName)
}

func TestWireGuardUpdateInterface(t *testing.T) {
	mgr := setup(t)

	interfaceName := persistence.InterfaceIdentifier("wg_test_001")
	defer mgr.DeleteInterface(interfaceName)

	err := mgr.CreateInterface(interfaceName)

	keys, err := mgr.GetFreshKeypair()
	assert.NoError(t, err)
	cfg := &persistence.InterfaceConfig{
		Identifier: interfaceName,
		KeyPair:    keys,
		ListenPort: 12567,
		Mtu:        1420,
		AddressStr: "10.98.87.76/24",
		Enabled:    true,
	}
	err = mgr.UpdateInterface(interfaceName, cfg)
	assert.NoError(t, err)

	// Validate that the interface has been updated
	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), interfaceName)
	assert.Contains(t, string(out), "10.98.87.76")
}

func TestWireGuardDisableInterface(t *testing.T) {
	mgr := setup(t)

	interfaceName := persistence.InterfaceIdentifier("wg_test_001")
	defer mgr.DeleteInterface(interfaceName)

	err := mgr.CreateInterface(interfaceName)

	keys, err := mgr.GetFreshKeypair()
	assert.NoError(t, err)
	cfg := &persistence.InterfaceConfig{
		Identifier: interfaceName,
		KeyPair:    keys,
		ListenPort: 12567,
		Mtu:        1420,
		AddressStr: "10.98.87.76/24",
		Enabled:    false,
	}
	err = mgr.UpdateInterface(interfaceName, cfg)
	assert.NoError(t, err)

	// Validate that the interface has been updated
	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), interfaceName)
	assert.Contains(t, string(out), "10.98.87.76")
	assert.Contains(t, string(out), "state DOWN")
}

func TestWireGuardEnableInterface(t *testing.T) {
	mgr := setup(t)

	interfaceName := persistence.InterfaceIdentifier("wg_test_001")
	defer mgr.DeleteInterface(interfaceName)

	err := mgr.CreateInterface(interfaceName)

	keys, err := mgr.GetFreshKeypair()
	assert.NoError(t, err)
	cfg := &persistence.InterfaceConfig{
		Identifier: interfaceName,
		KeyPair:    keys,
		ListenPort: 12567,
		Mtu:        1420,
		AddressStr: "10.98.87.76/24",
		Enabled:    false,
	}
	err = mgr.UpdateInterface(interfaceName, cfg)
	assert.NoError(t, err)

	cfg.Enabled = true
	err = mgr.UpdateInterface(interfaceName, cfg)
	assert.NoError(t, err)

	// Validate that the interface has been updated
	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), interfaceName)
	assert.Contains(t, string(out), "10.98.87.76")
	assert.Contains(t, string(out), "state U") // Can be UNKNOWN or UP
}
