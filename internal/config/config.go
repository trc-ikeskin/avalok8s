package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	QueryInterval time.Duration `env:"QUERY_INTERVAL" envDefault:"5s"`
}

func NewConfig() (Config, error) {
	c := Config{}

	err := env.Parse(&c)
	if err != nil {
		return Config{}, err
	}

	return c, nil
}
