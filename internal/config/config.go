// Package config loads application configuration from environment variables
// and command-line flags. Environment variables take precedence over flags,
// and flags take precedence over hard-coded defaults.
package config

import (
	"flag"

	"github.com/caarlos0/env/v6"
)

// Config holds all application configuration values.
// Each field can be set via the corresponding environment variable (highest priority),
// a command-line flag, or falls back to the built-in default.
type Config struct {
	// ServerAddress is the TCP address the HTTP server listens on (flag -a, env SERVER_ADDRESS).
	ServerAddress string `env:"SERVER_ADDRESS"`

	// BaseURL is the public base URL prepended to every short ID (flag -b, env BASE_URL).
	BaseURL string `env:"BASE_URL"`

	// FileStoragePath is the path to the JSON file used for persistent storage
	// when no database is configured (flag -f, env FILE_STORAGE_PATH).
	// An empty value disables file storage.
	FileStoragePath string `env:"FILE_STORAGE_PATH"`

	// DatabaseDSN is the PostgreSQL connection string (flag -d, env DATABASE_DSN).
	// An empty value disables database storage.
	DatabaseDSN string `env:"DATABASE_DSN"`

	// AuditFile is the path to the audit log file (flag --audit-file, env AUDIT_FILE).
	// Audit events are appended as JSON lines. An empty value disables this sink.
	AuditFile string `env:"AUDIT_FILE"`

	// AuditURL is the URL of a remote audit server (flag --audit-url, env AUDIT_URL).
	// Audit events are POSTed as JSON. An empty value disables this sink.
	AuditURL string `env:"AUDIT_URL"`

	// EnableHTTPS starts the server in TLS mode using a self-signed certificate
	// (flag -s, env ENABLE_HTTPS).
	EnableHTTPS bool `env:"ENABLE_HTTPS"`
}

// Load reads configuration in priority order: environment variables > flags > defaults.
// It must be called once at program startup, before any flags are parsed elsewhere.
func Load() *Config {
	cfg := &Config{}

	env.Parse(cfg)

	// Capture env values before flag.Parse() overwrites cfg fields with flag defaults.
	envServerAddress := cfg.ServerAddress
	envBaseURL := cfg.BaseURL
	envFileStoragePath := cfg.FileStoragePath
	envDatabaseDSN := cfg.DatabaseDSN
	envAuditFile := cfg.AuditFile
	envAuditURL := cfg.AuditURL
	envEnableHTTPS := cfg.EnableHTTPS

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for shortened URLs")
	flag.StringVar(&cfg.FileStoragePath, "f", "/tmp/shortener-db.json", "File storage path")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "Database connection string (DSN)")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "Path to audit log file (disabled if empty)")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "URL of remote audit server (disabled if empty)")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "Enable HTTPS with a self-signed certificate")

	flag.Parse()

	// Restore env values — they win over any flag defaults.
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
	if envEnableHTTPS {
		cfg.EnableHTTPS = true
	}

	return cfg
}
