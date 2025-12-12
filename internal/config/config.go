package config

import (
	"flag"
	"github.com/caarlos0/env/v6"
)

// Config holds application configuration
type Config struct {
	ServerAddress string `env:"SERVER_ADDRESS"`
	BaseURL       string `env:"BASE_URL"`
}

func Load() *Config {
	cfg := &Config{}

	_ = env.Parse(cfg)

	if cfg.ServerAddress == "" {
		flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	}

	if cfg.BaseURL == "" {
		flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for shortened URLs")
	}

	flag.Parse()

	return cfg
}
