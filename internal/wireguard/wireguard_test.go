//go:build !integration
// +build !integration

package wireguard

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vishvananda/netlink"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
)

func TestWgCtrlManager_GetInterfaces(t *testing.T) {
	tests := []struct {
		name    string
		manager *wgCtrlManager
		want    []*persistence.InterfaceConfig
		wantErr bool
	}{
		{
			name: "NoInterface",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{},
			},
			want:    []*persistence.InterfaceConfig{},
			wantErr: false,
		},
		{
			name: "Normal",
			manager: &wgCtrlManager{
				mux: sync.RWMutex{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{
					"wg0": {Identifier: "wg0"},
					"wg2": {Identifier: "wg2"},
					"wg1": {Identifier: "wg1"},
				},
			},
			want: []*persistence.InterfaceConfig{
				{Identifier: "wg0"},
				{Identifier: "wg1"},
				{Identifier: "wg2"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.manager.GetInterfaces()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetInterfaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetInterfaces() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWgCtrlManager_CreateInterface(t *testing.T) {
	tests := []struct {
		name      string
		manager   *wgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      persistence.InterfaceIdentifier
		wantErr   bool
	}{
		{
			name: "AlreadyExisting",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {},
			args:      persistence.InterfaceIdentifier("wg0"),
			wantErr:   true,
		},
		{
			name: "LinkAddFailure",
			manager: &wgCtrlManager{
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
			manager: &wgCtrlManager{
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
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: make(map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig),
				peers:      make(map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig),
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkAdd", mock.Anything).Return(nil)
				nl.On("LinkSetUp", mock.Anything).Return(nil)
				st.On("SaveInterface", mock.Anything).Return(errors.New("failure"))
			},
			args:    persistence.InterfaceIdentifier("wg0"),
			wantErr: true,
		},
		{
			name: "Success",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: make(map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig),
				peers:      make(map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig),
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkAdd", mock.Anything).Return(nil)
				nl.On("LinkSetUp", mock.Anything).Return(nil)
				st.On("SaveInterface", mock.Anything).Return(nil)
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
		manager   *wgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      persistence.InterfaceIdentifier
		wantErr   bool
	}{
		{
			name: "NonExisting",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {},
			args:      "wg0",
			wantErr:   true,
		},
		{
			name: "LowLevelFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkDel", mock.Anything).Return(errors.New("failure"))
			},
			args:    "wg0",
			wantErr: true,
		},
		{
			name: "PersistenceFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkDel", mock.Anything).Return(nil)
				st.On("DeleteInterface", mock.Anything).Return(errors.New("failure"))
			},
			args:    "wg0",
			wantErr: true,
		},
		{
			name: "PeerPersistenceFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {"peer0": {Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"}}},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkDel", mock.Anything).Return(nil)
				st.On("DeleteInterface", persistence.InterfaceIdentifier("wg0")).Return(nil)
				st.On("DeletePeer", persistence.PeerIdentifier("peer0"), persistence.InterfaceIdentifier("wg0")).Return(errors.New("failure"))
			},
			args:    "wg0",
			wantErr: true,
		},
		{
			name: "Success",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {"peer0": {Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"}}},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkDel", mock.Anything).Return(nil)
				st.On("DeleteInterface", persistence.InterfaceIdentifier("wg0")).Return(nil)
				st.On("DeletePeer", persistence.PeerIdentifier("peer0"), persistence.InterfaceIdentifier("wg0")).Return(nil)
			},
			args:    "wg0",
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
			if err := tt.manager.DeleteInterface(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("DeleteInterface() error = %v, wantErr %v", err, tt.wantErr)
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
		cfg *persistence.InterfaceConfig
	}
	tests := []struct {
		name      string
		manager   *wgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      args
		wantErr   bool
	}{
		{
			name: "NonExistent",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {},
			args: args{
				id: "wg0",
			},
			wantErr: true,
		},
		{
			name: "NonExistentLowLevel",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				nl.On("LinkByName", "wg0").Return(nil, errors.New("failure"))
			},
			args: args{
				id:  "wg0",
				cfg: &persistence.InterfaceConfig{},
			},
			wantErr: true,
		},
		{
			name: "SuccessEnabled",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				virtLink := &netlink.GenericLink{LinkType: "wireguard"}
				nl.On("LinkByName", "wg0").Return(virtLink, nil)
				nl.On("LinkSetMTU", virtLink, 234).Return(nil)
				nl.On("AddrReplace", virtLink, mock.MatchedBy(func(addr *netlink.Addr) bool {
					return addr.String() == "1.2.3.4/24"
				})).Return(nil)
				nl.On("AddrAdd", virtLink, mock.MatchedBy(func(addr *netlink.Addr) bool {
					return addr.String() == "10.0.0.2/24"
				})).Return(nil)
				wg.On("ConfigureDevice", "wg0", mock.Anything).Return(nil)
				nl.On("LinkSetUp", virtLink).Return(nil)
				st.On("SaveInterface", mock.Anything).Return(nil)
			},
			args: args{
				id: "wg0",
				cfg: &persistence.InterfaceConfig{
					Mtu: 234, AddressStr: "10.0.0.2/24,1.2.3.4/24", Enabled: true,
					KeyPair: persistence.KeyPair{PrivateKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI="},
				},
			},
			wantErr: false,
		},
		{
			name: "SuccessDisabled",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				virtLink := &netlink.GenericLink{LinkType: "wireguard"}
				nl.On("LinkByName", "wg0").Return(virtLink, nil)
				nl.On("LinkSetMTU", virtLink, 234).Return(nil)
				nl.On("AddrReplace", virtLink, mock.MatchedBy(func(addr *netlink.Addr) bool {
					return addr.String() == "1.2.3.4/24"
				})).Return(nil)
				nl.On("AddrAdd", virtLink, mock.MatchedBy(func(addr *netlink.Addr) bool {
					return addr.String() == "10.0.0.2/24"
				})).Return(nil)
				wg.On("ConfigureDevice", "wg0", mock.Anything).Return(nil)
				nl.On("LinkSetDown", virtLink).Return(nil)
				st.On("SaveInterface", mock.Anything).Return(nil)
			},
			args: args{
				id: "wg0",
				cfg: &persistence.InterfaceConfig{
					Mtu: 234, AddressStr: "10.0.0.2/24,1.2.3.4/24", Enabled: false,
					KeyPair: persistence.KeyPair{PrivateKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI="},
				},
			},
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
			if err := tt.manager.UpdateInterface(tt.args.id, tt.args.cfg); (err != nil) != tt.wantErr {
				t.Errorf("UpdateInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.manager.wg.(*MockWireGuardClient).AssertExpectations(t)
			tt.manager.nl.(*MockNetlinkClient).AssertExpectations(t)
			tt.manager.store.(*MockWireGuardStore).AssertExpectations(t)
		})
	}
}

func TestWgCtrlManager_ApplyDefaultConfigs(t *testing.T) {
	type args struct {
		id persistence.InterfaceIdentifier
	}
	tests := []struct {
		name      string
		manager   *wgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      args
		wantErr   bool
	}{
		{
			name: "NoInterface",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {},
			args: args{
				id: "wg0",
			},
			wantErr: true,
		},
		{
			name: "PersistenceFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {Identifier: "wg0"}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Identifier: "peer0", Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"}},
					},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				st.On("SavePeer", mock.Anything, persistence.InterfaceIdentifier("wg0")).Return(errors.New("failure"))
			},
			args: args{
				id: "wg0",
			},
			wantErr: true,
		},
		{
			name: "Success",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {Identifier: "wg0"}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": {Identifier: "peer0", Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"}},
					},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				st.On("SavePeer", mock.Anything, persistence.InterfaceIdentifier("wg0")).Return(nil)
			},
			args: args{
				id: "wg0",
			},
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
			if err := tt.manager.ApplyDefaultConfigs(tt.args.id); (err != nil) != tt.wantErr {
				t.Errorf("ApplyDefaultConfigs() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.manager.wg.(*MockWireGuardClient).AssertExpectations(t)
			tt.manager.nl.(*MockNetlinkClient).AssertExpectations(t)
			tt.manager.store.(*MockWireGuardStore).AssertExpectations(t)
		})
	}
}

func TestWgCtrlManager_GetPeers(t *testing.T) {
	tests := []struct {
		name        string
		manager     *wgCtrlManager
		interfaceId persistence.InterfaceIdentifier
		want        []*persistence.PeerConfig
		wantErr     bool
	}{
		{
			name: "NoInterface",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{},
			},
			interfaceId: "wg0",
			want:        nil,
			wantErr:     true,
		},
		{
			name: "Normal",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {
						"peer0": &persistence.PeerConfig{Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"}},
						"peer1": &persistence.PeerConfig{Interface: &persistence.PeerInterfaceConfig{Identifier: "wg1"}},
					},
				},
			},
			interfaceId: "wg0",
			want: []*persistence.PeerConfig{
				{Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"}},
				{Interface: &persistence.PeerInterfaceConfig{Identifier: "wg1"}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.manager.GetPeers(tt.interfaceId)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPeers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !assert.Equal(t, got, tt.want) {
				t.Errorf("GetPeers() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWgCtrlManager_SavePeers(t *testing.T) {
	tests := []struct {
		name      string
		manager   *wgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      []*persistence.PeerConfig
		wantErr   bool
	}{
		{
			name: "NoInterface",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {},
			args:      []*persistence.PeerConfig{{Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"}}},
			wantErr:   true,
		},
		{
			name: "ConfigGenerationFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {},
			args:      []*persistence.PeerConfig{{Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"}}},
			wantErr:   true,
		},
		{
			name: "WireGuardFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers:      nil,
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				wg.On("ConfigureDevice", "wg0", mock.Anything).Return(errors.New("failure"))
			},
			args: []*persistence.PeerConfig{
				{
					KeyPair:   persistence.KeyPair{PublicKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=", PrivateKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI="},
					Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"},
				},
			},
			wantErr: true,
		},
		{
			name: "PersistenceFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				wg.On("ConfigureDevice", "wg0", mock.Anything).Return(nil)
				st.On("SavePeer", mock.Anything, persistence.InterfaceIdentifier("wg0")).Return(errors.New("failure"))
			},
			args: []*persistence.PeerConfig{
				{
					KeyPair:   persistence.KeyPair{PublicKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=", PrivateKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI="},
					Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"},
				},
			},
			wantErr: true,
		},
		{
			name: "Success",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				wg.On("ConfigureDevice", "wg0", mock.Anything).Return(nil)
				st.On("SavePeer", mock.Anything, persistence.InterfaceIdentifier("wg0")).Return(nil)
			},
			args: []*persistence.PeerConfig{
				{
					KeyPair:   persistence.KeyPair{PublicKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=", PrivateKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI="},
					Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"},
				},
			},
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
			if err := tt.manager.SavePeers(tt.args...); (err != nil) != tt.wantErr {
				t.Errorf("SavePeers() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.manager.wg.(*MockWireGuardClient).AssertExpectations(t)
			tt.manager.nl.(*MockNetlinkClient).AssertExpectations(t)
			tt.manager.store.(*MockWireGuardStore).AssertExpectations(t)
		})
	}
}

func TestWgCtrlManager_RemovePeer(t *testing.T) {
	tests := []struct {
		name      string
		manager   *wgCtrlManager
		mockSetup func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore)
		args      persistence.PeerIdentifier
		wantErr   bool
	}{
		{
			name: "NoPeer",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{},
				peers:      map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {},
			args:      "peer0",
			wantErr:   true,
		},
		{
			name: "WireGuardFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {"peer0": {
						KeyPair: persistence.KeyPair{
							PublicKey:  "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=",
							PrivateKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=",
						},
						Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"},
					}},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				wg.On("ConfigureDevice", "wg0", mock.Anything).Return(errors.New("failure"))
			},
			args:    "peer0",
			wantErr: true,
		},
		{
			name: "PersistenceFailure",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {"peer0": {
						KeyPair: persistence.KeyPair{
							PublicKey:  "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=",
							PrivateKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=",
						},
						Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"},
					}},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				wg.On("ConfigureDevice", "wg0", mock.Anything).Return(nil)
				st.On("DeletePeer", persistence.PeerIdentifier("peer0"), persistence.InterfaceIdentifier("wg0")).Return(errors.New("failure"))
			},
			args:    "peer0",
			wantErr: true,
		},
		{
			name: "Success",
			manager: &wgCtrlManager{
				mux:        sync.RWMutex{},
				wg:         &MockWireGuardClient{},
				nl:         &MockNetlinkClient{},
				store:      &MockWireGuardStore{},
				interfaces: map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig{"wg0": {}},
				peers: map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig{
					"wg0": {"peer0": {
						KeyPair: persistence.KeyPair{
							PublicKey:  "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=",
							PrivateKey: "pcDxSxSZp5x87cNoRJaHdAOzxrxDfDUn7pGmrY/AmzI=",
						},
						Interface: &persistence.PeerInterfaceConfig{Identifier: "wg0"},
					}},
				},
			},
			mockSetup: func(wg *MockWireGuardClient, nl *MockNetlinkClient, st *MockWireGuardStore) {
				wg.On("ConfigureDevice", "wg0", mock.Anything).Return(nil)
				st.On("DeletePeer", persistence.PeerIdentifier("peer0"), persistence.InterfaceIdentifier("wg0")).Return(nil)
			},
			args:    "peer0",
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
			if err := tt.manager.RemovePeer(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("RemovePeer() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.manager.wg.(*MockWireGuardClient).AssertExpectations(t)
			tt.manager.nl.(*MockNetlinkClient).AssertExpectations(t)
			tt.manager.store.(*MockWireGuardStore).AssertExpectations(t)
		})
	}
}
