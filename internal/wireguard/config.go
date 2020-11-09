package wireguard

type Config struct {
	DeviceName      string `yaml:"device" envconfig:"WG_DEVICE"`
	WireGuardConfig string `yaml:"configFile" envconfig:"WG_CONFIG_FILE"` // optional, if set, updates will be written to this file
}
