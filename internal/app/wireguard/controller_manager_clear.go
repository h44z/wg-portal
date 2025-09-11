package wireguard

import (
	"context"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ClearPeers повністю очищає peers на інтерфейсі (ReplacePeers=true з порожнім списком).
func (m *ControllerManager) ClearPeers(_ context.Context, iface string) error {
    c, err := wgctrl.New()
    if err != nil { return err }
    defer c.Close()

    return c.ConfigureDevice(iface, wgtypes.Config{
        ReplacePeers: true,
        Peers:        []wgtypes.PeerConfig{},
    })
}