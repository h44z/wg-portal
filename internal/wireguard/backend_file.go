package wireguard

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type FileBackend struct {
	configurationPath string
	fileGenerator     ConfigFileGenerator
}

func NewFileBackend(configStoragePath string, fileGenerator ConfigFileGenerator) (*FileBackend, error) {
	backend := &FileBackend{configurationPath: configStoragePath, fileGenerator: fileGenerator}
	return backend, nil
}

func (f FileBackend) SaveInterface(cfg InterfaceConfig, peers []PeerConfig) error {
	configContents, err := f.fileGenerator.GetInterfaceConfig(cfg, peers)
	if err != nil {
		return errors.Wrapf(err, "failed to generate config file contents for %s", cfg.DeviceName)
	}

	configFilePath := filepath.Join(f.configurationPath, string(cfg.DeviceName)+".conf")
	configFile, err := os.OpenFile(configFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return errors.Wrapf(err, "failed to create config file for %s", cfg.DeviceName)
	}
	defer configFile.Close()

	_, err = io.Copy(configFile, configContents)
	if err != nil {
		return errors.Wrapf(err, "failed to write config file for %s", cfg.DeviceName)
	}

	return nil
}

func (f FileBackend) SavePeer(_ PeerConfig, _ InterfaceConfig) error {
	return nil // the file backend will only store changed interfaces
}

func (f FileBackend) DeleteInterface(cfg InterfaceConfig, _ []PeerConfig) error {
	configFilePath := filepath.Join(f.configurationPath, string(cfg.DeviceName)+".conf")

	err := os.Remove(configFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to delete config file for %s", cfg.DeviceName)
	}
	return nil
}

func (f FileBackend) DeletePeer(_ PeerConfig, _ InterfaceConfig) error {
	return nil // the file backend will only store changed interfaces
}

func (f FileBackend) Load(identifier DeviceIdentifier) (InterfaceConfig, []PeerConfig, error) {
	panic("implement me")
}

func (f FileBackend) LoadAll(ignored ...DeviceIdentifier) (map[InterfaceConfig][]PeerConfig, error) {
	panic("implement me")
}
