package wireguard

import (
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

//
// -- WireGuard mock
//

type MockWireGuardClient struct {
	mock.Mock
}

func (m *MockWireGuardClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockWireGuardClient) Devices() ([]*wgtypes.Device, error) {
	args := m.Called()
	return args.Get(0).([]*wgtypes.Device), args.Error(1)
}

func (m *MockWireGuardClient) Device(name string) (*wgtypes.Device, error) {
	args := m.Called(name)
	return args.Get(0).(*wgtypes.Device), args.Error(1)
}

func (m *MockWireGuardClient) ConfigureDevice(name string, cfg wgtypes.Config) error {
	args := m.Called(name, cfg)
	return args.Error(0)
}

//
// -- Netlink mock
//

type MockNetlinkClient struct {
	mock.Mock
}

func (m *MockNetlinkClient) LinkAdd(link netlink.Link) error {
	args := m.Called(link)
	return args.Error(0)
}

func (m *MockNetlinkClient) LinkDel(link netlink.Link) error {
	args := m.Called(link)
	return args.Error(0)
}

func (m *MockNetlinkClient) LinkByName(name string) (netlink.Link, error) {
	args := m.Called(name)
	if args.Get(0) != nil {
		return args.Get(0).(netlink.Link), args.Error(1)
	} else {
		return nil, args.Error(1)
	}
}

func (m *MockNetlinkClient) LinkSetUp(link netlink.Link) error {
	args := m.Called(link)
	return args.Error(0)
}

func (m *MockNetlinkClient) LinkSetDown(link netlink.Link) error {
	args := m.Called(link)
	return args.Error(0)
}

func (m *MockNetlinkClient) LinkSetMTU(link netlink.Link, mtu int) error {
	args := m.Called(link, mtu)
	return args.Error(0)
}

func (m *MockNetlinkClient) AddrReplace(link netlink.Link, addr *netlink.Addr) error {
	args := m.Called(link, addr)
	return args.Error(0)
}

func (m *MockNetlinkClient) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	args := m.Called(link, addr)
	return args.Error(0)
}

func (m *MockNetlinkClient) AddrList(link netlink.Link) ([]netlink.Addr, error) {
	args := m.Called(link)
	return args.Get(0).([]netlink.Addr), args.Error(1)
}

//
// -- WireGuard Store mock
//

type MockWireGuardStore struct {
	mock.Mock
}

func (w *MockWireGuardStore) GetAvailableInterfaces() ([]persistence.InterfaceIdentifier, error) {
	args := w.Called()
	return args.Get(0).([]persistence.InterfaceIdentifier), args.Error(1)
}

func (w *MockWireGuardStore) GetAllInterfaces(interfaceIdentifiers ...persistence.InterfaceIdentifier) (map[persistence.InterfaceConfig][]persistence.PeerConfig, error) {
	args := w.Called(interfaceIdentifiers)
	return args.Get(0).(map[persistence.InterfaceConfig][]persistence.PeerConfig), args.Error(1)
}

func (w *MockWireGuardStore) GetInterface(identifier persistence.InterfaceIdentifier) (persistence.InterfaceConfig, []persistence.PeerConfig, error) {
	args := w.Called(identifier)
	return args.Get(0).(persistence.InterfaceConfig), args.Get(1).([]persistence.PeerConfig), args.Error(2)
}

func (w *MockWireGuardStore) SaveInterface(cfg persistence.InterfaceConfig) error {
	args := w.Called(cfg)
	return args.Error(0)
}

func (w *MockWireGuardStore) SavePeer(peer persistence.PeerConfig) error {
	args := w.Called(peer)
	return args.Error(0)
}

func (w *MockWireGuardStore) DeleteInterface(identifier persistence.InterfaceIdentifier) error {
	args := w.Called(identifier)
	return args.Error(0)
}

func (w *MockWireGuardStore) DeletePeer(peer persistence.PeerIdentifier) error {
	args := w.Called(peer)
	return args.Error(0)
}
