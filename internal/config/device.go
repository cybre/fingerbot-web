package config

type Device struct {
	DeviceAddress  string `envconfig:"DEVICE_ADDRESS" required:"true"`
	DeviceID       string `envconfig:"DEVICE_ID" required:"true"`
	DeviceLocalKey string `envconfig:"DEVICE_LOCAL_KEY" required:"true"`
	DeviceUUID     string `envconfig:"DEVICE_UUID" required:"true"`
}
