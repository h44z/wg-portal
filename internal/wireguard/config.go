package wireguard

import "github.com/h44z/wg-portal/internal/common"

type Config struct {
	DeviceNames         []string `yaml:"devices" envconfig:"WG_DEVICES"`             // managed devices
	DefaultDeviceName   string   `yaml:"devices" envconfig:"WG_DEFAULT_DEVICE"`      // this device is used for auto-created peers, use GetDefaultDeviceName() to access this field
	ConfigDirectoryPath string   `yaml:"configDirectory" envconfig:"WG_CONFIG_PATH"` // optional, if set, updates will be written to this path, filename: <devicename>.conf
	ManageIPAddresses   bool     `yaml:"manageIPAddresses" envconfig:"MANAGE_IPS"`   // handle ip-address setup of interface
}

func (c Config) GetDefaultDeviceName() string {
	if c.DefaultDeviceName == "" || !common.ListContains(c.DeviceNames, c.DefaultDeviceName) {
		return c.DeviceNames[0]
	}
	return c.DefaultDeviceName
}
