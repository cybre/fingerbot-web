package config

type Device struct {
	DeviceAddress string `envconfig:"DEVICE_ADDRESS" required:"true"`
	DeviceID      string `envconfig:"DEVICE_ID" required:"true"`
}
