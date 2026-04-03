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
	DatabaseDSN     string `env:"DATABASE_DSN"`
	AuditFile       string `env:"AUDIT_FILE"`
	AuditURL        string `env:"AUDIT_URL"`
}

func Load() *Config {
	cfg := &Config{}

	env.Parse(cfg)

	envServerAddress := cfg.ServerAddress
	envBaseURL := cfg.BaseURL
	envFileStoragePath := cfg.FileStoragePath
	envDatabaseDSN := cfg.DatabaseDSN
	envAuditFile := cfg.AuditFile
	envAuditURL := cfg.AuditURL

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for shortened URLs")
	flag.StringVar(&cfg.FileStoragePath, "f", "/tmp/shortener-db.json", "File storage path")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "Database connection string (DSN)")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "Path to audit log file (disabled if empty)")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "URL of remote audit server (disabled if empty)")

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
	if envDatabaseDSN != "" {
		cfg.DatabaseDSN = envDatabaseDSN
	}
	if envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	}
	if envAuditURL != "" {
		cfg.AuditURL = envAuditURL
	}

	return cfg
}
