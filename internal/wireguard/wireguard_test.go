//go:build !integration
// +build !integration

package wireguard

import (
	"reflect"
	"sync"
	"testing"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
)

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
				st.On("SaveInterface", mock.Anything, mock.Anything).Return(errors.New("failure"))
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
		manager   *wgCtrlManager
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
		manager   *wgCtrlManager
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
		cfg *persistence.InterfaceConfig
	}
	tests := []struct {
		name      string
		manager   *wgCtrlManager
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
