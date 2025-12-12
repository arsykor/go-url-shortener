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

	envServerAddress := cfg.ServerAddress
	envBaseURL := cfg.BaseURL

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for shortened URLs")

	flag.Parse()

	if envServerAddress != "" {
		cfg.ServerAddress = envServerAddress
	}

	if envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}

	return cfg
}
