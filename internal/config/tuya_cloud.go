package config

type TuyaCloud struct {
	TuyaCloudURL       string `envconfig:"TUYA_CLOUD_URL" default:"https://openapi.tuyaeu.com"`
	TuyaCloudAccessID  string `envconfig:"TUYA_CLOUD_ACCESS_ID" required:"true"`
	TuyaCloudAccessKey string `envconfig:"TUYA_CLOUD_ACCESS_KEY" required:"true"`
}
