//go:build !integration
// +build !integration

package wireguard

import (
	"net"
	"reflect"
	"sync"
	"testing"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
)

func TestWgCtrlManager_CreateInterface(t *testing.T) {
	tests := []struct {
		name      string
		manager   *WgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      persistence.InterfaceIdentifier
		wantErr   bool
	}{
		{
			name: "AlreadyExisting",
			manager: &WgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {},
			args:      persistence.InterfaceIdentifier("wg0"),
			wantErr:   true,
		},
		{
			name: "LinkAddFailure",
			manager: &WgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: nil,
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkAdd", mock.Anything).Return(errors.New("failure"))
			},
			args:    persistence.InterfaceIdentifier("wg0"),
			wantErr: true,
		},
		{
			name: "LinkSetupFailure",
			manager: &WgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: nil,
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkAdd", mock.Anything).Return(nil)
				nl.On("LinkSetUp", mock.Anything).Return(errors.New("failure"))
			},
			args:    persistence.InterfaceIdentifier("wg0"),
			wantErr: true,
		},
		{
			name: "PersistenceFailure",
			manager: &WgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: make(map[persistence.InterfaceIdentifier]persistence.InterfaceConfig),
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkAdd", mock.Anything).Return(nil)
				nl.On("LinkSetUp", mock.Anything).Return(nil)
				st.On("SaveInterface", mock.Anything, mock.Anything).Return(errors.New("failure"))
			},
			args:    persistence.InterfaceIdentifier("wg0"),
			wantErr: true,
		},
		{
			name: "Success",
			manager: &WgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: make(map[persistence.InterfaceIdentifier]persistence.InterfaceConfig),
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkAdd", mock.Anything).Return(nil)
				nl.On("LinkSetUp", mock.Anything).Return(nil)
				st.On("SaveInterface", mock.Anything, mock.Anything).Return(nil)
			},
			args:    persistence.InterfaceIdentifier("wg0"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(
				tt.manager.wg.(*MockWireGuardClient),
				tt.manager.nl.(*MockNetlinkClient),
				tt.manager.store.(*MockWireGuardStore),
			)
			if err := tt.manager.CreateInterface(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("CreateInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.manager.wg.(*MockWireGuardClient).AssertExpectations(t)
			tt.manager.nl.(*MockNetlinkClient).AssertExpectations(t)
			tt.manager.store.(*MockWireGuardStore).AssertExpectations(t)
		})
	}
}

func TestWgCtrlManager_DeleteInterface(t *testing.T) {
	tests := []struct {
		name      string
		manager   *WgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      persistence.InterfaceIdentifier
		wantErr   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(
				tt.manager.wg.(*MockWireGuardClient),
				tt.manager.nl.(*MockNetlinkClient),
				tt.manager.store.(*MockWireGuardStore),
			)
			if err := tt.manager.DeleteInterface(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("DeleteInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.manager.wg.(*MockWireGuardClient).AssertExpectations(t)
			tt.manager.nl.(*MockNetlinkClient).AssertExpectations(t)
			tt.manager.store.(*MockWireGuardStore).AssertExpectations(t)
		})
	}
}

func TestWgCtrlManager_GetInterfaces(t *testing.T) {
	tests := []struct {
		name      string
		manager   *WgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		want      []persistence.InterfaceConfig
		wantErr   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(
				tt.manager.wg.(*MockWireGuardClient),
				tt.manager.nl.(*MockNetlinkClient),
				tt.manager.store.(*MockWireGuardStore),
			)
			got, err := tt.manager.GetInterfaces()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetInterfaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetInterfaces() got = %v, want %v", got, tt.want)
			}
			tt.manager.wg.(*MockWireGuardClient).AssertExpectations(t)
			tt.manager.nl.(*MockNetlinkClient).AssertExpectations(t)
			tt.manager.store.(*MockWireGuardStore).AssertExpectations(t)
		})
	}
}

func TestWgCtrlManager_UpdateInterface(t *testing.T) {
	type args struct {
		id  persistence.InterfaceIdentifier
		cfg persistence.InterfaceConfig
	}
	tests := []struct {
		name      string
		manager   *WgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      args
		wantErr   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(
				tt.manager.wg.(*MockWireGuardClient),
				tt.manager.nl.(*MockNetlinkClient),
				tt.manager.store.(*MockWireGuardStore),
			)
			if err := tt.manager.UpdateInterface(tt.args.id, tt.args.cfg); (err != nil) != tt.wantErr {
				t.Errorf("UpdateInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.manager.wg.(*MockWireGuardClient).AssertExpectations(t)
			tt.manager.nl.(*MockNetlinkClient).AssertExpectations(t)
			tt.manager.store.(*MockWireGuardStore).AssertExpectations(t)
		})
	}
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
