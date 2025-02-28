//go:build integration

package adapters

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/domain"
)

// setup WireGuard manager with no linked store
func setup(t *testing.T) *WgRepo {
	if getProcessOwner() != "root" {
		t.Fatalf("this tests need to be executed as root user")
	}

	repo := NewWireGuardRepository()

	return repo
}

func getProcessOwner() string {
	stdout, err := exec.Command("ps", "-o", "user=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return strings.TrimSpace(string(stdout))
}

func Test_wgRepository_GetInterfaces(t *testing.T) {
	mgr := setup(t)

	interfaceName := domain.InterfaceIdentifier("wg_test_001")
	defer mgr.DeleteInterface(context.Background(), interfaceName)
	err := mgr.SaveInterface(context.Background(), interfaceName, nil)
	require.NoError(t, err)

	interfaceName2 := domain.InterfaceIdentifier("wg_test_002")
	defer mgr.DeleteInterface(context.Background(), interfaceName2)
	err = mgr.SaveInterface(context.Background(), interfaceName2, nil)
	require.NoError(t, err)

	interfaces, err := mgr.GetInterfaces(context.Background())
	assert.NoError(t, err)
	assert.Len(t, interfaces, 2)
	for _, iface := range interfaces {
		assert.True(t, iface.Identifier == interfaceName || iface.Identifier == interfaceName2)
	}
}

func TestWireGuardCreateInterface(t *testing.T) {
	mgr := setup(t)

	interfaceName := domain.InterfaceIdentifier("wg_test_001")
	ipAddress := "10.11.12.13"
	ipV6Address := "1337:d34d:b33f::2"
	defer mgr.DeleteInterface(context.Background(), interfaceName)

	err := mgr.SaveInterface(context.Background(), interfaceName,
		func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
			pi.Addresses = []domain.Cidr{
				domain.CidrFromIpNet(net.IPNet{IP: net.ParseIP(ipAddress), Mask: net.CIDRMask(24, 32)}),
				domain.CidrFromIpNet(net.IPNet{IP: net.ParseIP(ipV6Address), Mask: net.CIDRMask(64, 128)}),
			}
			return pi, nil
		})
	assert.NoError(t, err)

	// Validate that the interface has been created
	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), interfaceName)
	assert.Contains(t, string(out), ipAddress)
	assert.Contains(t, string(out), ipV6Address)
}

func TestWireGuardUpdateInterface(t *testing.T) {
	mgr := setup(t)

	interfaceName := domain.InterfaceIdentifier("wg_test_001")
	defer mgr.DeleteInterface(context.Background(), interfaceName)

	err := mgr.SaveInterface(context.Background(), interfaceName, nil)
	require.NoError(t, err)

	cmd := exec.Command("ip", "addr")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(out), interfaceName)

	ipAddress := "10.11.12.13"
	ipV6Address := "1337:d34d:b33f::2"
	err = mgr.SaveInterface(context.Background(), interfaceName,
		func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
			pi.Addresses = []domain.Cidr{
				domain.CidrFromIpNet(net.IPNet{IP: net.ParseIP(ipAddress), Mask: net.CIDRMask(24, 32)}),
				domain.CidrFromIpNet(net.IPNet{IP: net.ParseIP(ipV6Address), Mask: net.CIDRMask(64, 128)}),
			}
			return pi, nil
		})
	assert.NoError(t, err)

	// Validate that the interface has been updated
	cmd = exec.Command("ip", "addr")
	out, err = cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), interfaceName)
	assert.Contains(t, string(out), ipAddress)
	assert.Contains(t, string(out), ipV6Address)
}
