//go:build !integration
// +build !integration

package wireguard

import (
	"net"
	"reflect"
	"testing"

	"github.com/stretchr/testify/mock"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

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
	return args.Get(0).(netlink.Link), args.Error(1)
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

//
// ---------- Tests
//

func TestManagementUtil_GetFreshKeypair(t *testing.T) {
	m := ManagementUtil{}
	kp, err := m.GetFreshKeypair()
	assert.NoError(t, err)
	assert.NotEmpty(t, kp.PrivateKey)
	assert.NotEmpty(t, kp.PublicKey)
}

func TestManagementUtil_GetPreSharedKey(t *testing.T) {
	m := ManagementUtil{}
	psk, err := m.GetPreSharedKey()
	assert.NoError(t, err)
	assert.NotEmpty(t, psk)
}

func Test_parseIpAddressString(t *testing.T) {
	type args struct {
		addrStr string
	}
	var tests = []struct {
		name    string
		args    args
		want    []*netlink.Addr
		wantErr bool
	}{
		{
			name:    "Empty String",
			args:    args{},
			want:    []*netlink.Addr{},
			wantErr: false,
		},
		{
			name:    "Single IPv4",
			args:    args{addrStr: "123.123.123.123"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Malformed",
			args:    args{addrStr: "hello world"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Single IPv4 CIDR",
			args: args{addrStr: "123.123.123.123/24"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IPv4(123, 123, 123, 123),
					Mask: net.IPv4Mask(255, 255, 255, 0),
				},
			}},
			wantErr: false,
		},
		{
			name: "Multiple IPv4 CIDR",
			args: args{addrStr: "123.123.123.123/24, 200.201.202.203/16"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IPv4(123, 123, 123, 123),
					Mask: net.IPv4Mask(255, 255, 255, 0),
				},
			}, {
				IPNet: &net.IPNet{
					IP:   net.IPv4(200, 201, 202, 203),
					Mask: net.IPv4Mask(255, 255, 0, 0),
				},
			}},
			wantErr: false,
		},
		{
			name: "Single IPv6 CIDR",
			args: args{addrStr: "fe80::1/64"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IP{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0},
				},
			}},
			wantErr: false,
		},
		{
			name: "Multiple IPv6 CIDR",
			args: args{addrStr: "fe80::1/64 , 2130:d3ad::b33f/128"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IP{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0},
				},
			}, {
				IPNet: &net.IPNet{
					IP:   net.IP{0x21, 0x30, 0xd3, 0xad, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xb3, 0x3f},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
				},
			}},
			wantErr: false,
		},
		{
			name: "Mixed IPv4 and IPv6 CIDR",
			args: args{addrStr: "200.201.202.203/16,2130:d3ad::b33f/128"},
			want: []*netlink.Addr{{
				IPNet: &net.IPNet{
					IP:   net.IPv4(200, 201, 202, 203),
					Mask: net.IPv4Mask(255, 255, 0, 0),
				},
			}, {
				IPNet: &net.IPNet{
					IP:   net.IP{0x21, 0x30, 0xd3, 0xad, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xb3, 0x3f},
					Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
				},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIpAddressString(tt.args.addrStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIpAddressString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIpAddressString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManagementUtil_UpdateDevice(t *testing.T) {
	devName := DeviceIdentifier("wg668")
	wg := new(MockWireGuardClient)
	nl := new(MockNetlinkClient)

	// expectations
	nl.On("LinkByName", string(devName)).Return(&netlink.GenericLink{}, nil)
	nl.On("LinkSetMTU", mock.Anything, 1234).Return(nil)
	nl.On("AddrReplace", mock.Anything, mock.Anything).Return(nil)
	wg.On("ConfigureDevice", string(devName), mock.Anything).Return(nil)
	nl.On("LinkSetDown", mock.Anything).Return(nil)

	m := ManagementUtil{interfaces: map[DeviceIdentifier]InterfaceConfig{devName: {}}, nl: nl, wg: wg}

	err := m.UpdateDevice(devName, InterfaceConfig{AddressStr: "123.123.123.123/24", Mtu: 1234})
	assert.NoError(t, err)

	// assert that the expectations were met
	wg.AssertExpectations(t)
	nl.AssertExpectations(t)
}
