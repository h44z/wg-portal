package wireguard

type Config struct {
	DeviceName string `yaml:"device" envconfig:"WG_DEVICE"`
}
