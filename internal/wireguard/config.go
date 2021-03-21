package wireguard

type Config struct {
	DeviceNames         []string `yaml:"devices" envconfig:"WG_DEVICES"`             // managed devices
	DefaultDeviceName   string   `yaml:"devices" envconfig:"WG_DEFAULT_DEVICE"`      // this device is used for auto-created peers
	ConfigDirectoryPath string   `yaml:"configDirectory" envconfig:"WG_CONFIG_PATH"` // optional, if set, updates will be written to this path, filename: <devicename>.conf
	ManageIPAddresses   bool     `yaml:"manageIPAddresses" envconfig:"MANAGE_IPS"`   // handle ip-address setup of interface
}
