package lowlevel

import (
	"io"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// A WireGuardClient is a type which can control a WireGuard device.
type WireGuardClient interface {
	io.Closer
	Devices() ([]*wgtypes.Device, error)
	Device(name string) (*wgtypes.Device, error)
	ConfigureDevice(name string, cfg wgtypes.Config) error
}
