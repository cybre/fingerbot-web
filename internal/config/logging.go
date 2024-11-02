package config

type Logging struct {
	LoggingLevel     int  `envconfig:"LOG_LEVEL" default:"-4"`
	LoggingDevOutput bool `envconfig:"LOG_DEV_OUTPUT" default:"true"`
}
