package config

import (
	"errors"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sys/unix"
)

type Config struct {
	Service
	Logging
}

func Load(filenames ...string) (*Config, error) {
	if err := godotenv.Load(filenames...); err != nil && !errors.Is(err, unix.ENOENT) {
		return nil, fmt.Errorf("error reading configuration from .env file: %w", err)
	}

	config := &Config{}
	if err := envconfig.Process("", config); err != nil {
		return nil, fmt.Errorf("error loading configuration from env: %w", err)
	}

	return config, nil
}
