package wireguard

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockFileGenerator struct {
	mock.Mock
}

func (m *MockFileGenerator) GetInterfaceConfig(cfg InterfaceConfig, peers []PeerConfig) (io.Reader, error) {
	args := m.Called(cfg, peers)
	return args.Get(0).(io.Reader), args.Error(1)
}

func (m *MockFileGenerator) GetPeerConfig(peer PeerConfig, iface InterfaceConfig) (io.Reader, error) {
	args := m.Called(peer, iface)
	return args.Get(0).(io.Reader), args.Error(1)
}

func TestFileBackend_DeleteInterface(t *testing.T) {
	// setup
	tmpDir := os.TempDir()
	tmpFile, err := ioutil.TempFile(tmpDir, "wg*.conf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	f := FileBackend{
		configurationPath: tmpDir,
	}

	// Successful delete
	err = f.DeleteInterface(InterfaceConfig{
		DeviceName: DeviceIdentifier(strings.ReplaceAll(filepath.Base(tmpFile.Name()), ".conf", "")),
	}, nil)
	assert.NoError(t, err)

	// Unsuccessful delete
	err = f.DeleteInterface(InterfaceConfig{
		DeviceName: DeviceIdentifier(strings.ReplaceAll(filepath.Base(tmpFile.Name()), ".conf", "")),
	}, nil)
	assert.Error(t, err)
}

func TestFileBackend_DeletePeer(t *testing.T) {
	assert.NoError(t, FileBackend{}.DeletePeer(PeerConfig{}, InterfaceConfig{}))
}

func TestFileBackend_Load(t *testing.T) {
	type fields struct {
		configurationPath string
		fileGenerator     ConfigFileGenerator
	}
	type args struct {
		identifier DeviceIdentifier
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    InterfaceConfig
		want1   []PeerConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FileBackend{
				configurationPath: tt.fields.configurationPath,
				fileGenerator:     tt.fields.fileGenerator,
			}
			got, got1, err := f.Load(tt.args.identifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Load() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Load() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestFileBackend_LoadAll(t *testing.T) {
	type fields struct {
		configurationPath string
		fileGenerator     ConfigFileGenerator
	}
	type args struct {
		ignored []DeviceIdentifier
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[InterfaceConfig][]PeerConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FileBackend{
				configurationPath: tt.fields.configurationPath,
				fileGenerator:     tt.fields.fileGenerator,
			}
			got, err := f.LoadAll(tt.args.ignored...)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadAll() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileBackend_SaveInterface(t *testing.T) {
	// setup
	tmpDir := os.TempDir()
	tmpFile, err := ioutil.TempFile(tmpDir, "wg*.conf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	deviceName := strings.ReplaceAll(filepath.Base(tmpFile.Name()), ".conf", "")

	type fields struct {
		prepare func(m *mock.Mock)
	}
	type args struct {
		cfg   InterfaceConfig
		peers []PeerConfig
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "FileGeneratorError",
			fields: fields{
				prepare: func(m *mock.Mock) {
					m.On("GetInterfaceConfig", mock.Anything, mock.Anything).
						Return(&bytes.Buffer{}, errors.New("generr"))
				},
			},
			args:    args{},
			wantErr: true,
		},
		{
			name: "Success",
			fields: fields{
				prepare: func(m *mock.Mock) {
					m.On("GetInterfaceConfig", mock.Anything, mock.Anything).
						Return(bytes.NewBuffer([]byte("hello world")), nil)
				},
			},
			args: args{
				cfg:   InterfaceConfig{DeviceName: DeviceIdentifier(deviceName)},
				peers: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fg := new(MockFileGenerator)
			f := FileBackend{
				configurationPath: tmpDir,
				fileGenerator:     fg,
			}
			tt.fields.prepare(&fg.Mock)
			if err := f.SaveInterface(tt.args.cfg, tt.args.peers); (err != nil) != tt.wantErr {
				t.Errorf("SaveInterface() error = %v, wantErr %v", err, tt.wantErr)
			}

			fg.AssertExpectations(t)
		})
	}
}

func TestFileBackend_SavePeer(t *testing.T) {
	assert.NoError(t, FileBackend{}.SavePeer(PeerConfig{}, InterfaceConfig{}))
}

func TestNewFileBackend(t *testing.T) {
	got, err := NewFileBackend("testing", nil)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}
