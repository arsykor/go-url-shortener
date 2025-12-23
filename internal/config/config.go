package config

import (
	"flag"
	"github.com/caarlos0/env/v6"
)

// Config holds application configuration
type Config struct {
	ServerAddress   string `env:"SERVER_ADDRESS"`
	BaseURL         string `env:"BASE_URL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
}

func Load() *Config {
	cfg := &Config{}

	_ = env.Parse(cfg)

	envServerAddress := cfg.ServerAddress
	envBaseURL := cfg.BaseURL
	envFileStoragePath := cfg.FileStoragePath

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for shortened URLs")
	flag.StringVar(&cfg.FileStoragePath, "f", "/tmp/shortener-db.json", "File storage path")

	flag.Parse()

	// Priority: env > flag > default
	if envServerAddress != "" {
		cfg.ServerAddress = envServerAddress
	}

	if envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}

	if envFileStoragePath != "" {
		cfg.FileStoragePath = envFileStoragePath
	}

	return cfg
}
