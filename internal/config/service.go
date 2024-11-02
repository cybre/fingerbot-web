package config

type Service struct {
	ServicePort string `envconfig:"SERVICE_PORT" default:"8080"`
}
