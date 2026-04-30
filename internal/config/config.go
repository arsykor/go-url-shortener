// Package config loads application configuration from environment variables,
// command-line flags, and an optional JSON config file.
// Priority (highest → lowest): env vars > flags > JSON file > built-in defaults.
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/caarlos0/env/v6"
)

// Config holds all application configuration values.
// Each field can be set via the corresponding environment variable (highest priority),
// a command-line flag, a JSON config file, or falls back to the built-in default.
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

// fileConfig mirrors Config for JSON unmarshalling.
// Pointer fields let us distinguish "absent" from a zero value.
type fileConfig struct {
	ServerAddress   string `json:"server_address"`
	BaseURL         string `json:"base_url"`
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`
	AuditFile       string `json:"audit_file"`
	AuditURL        string `json:"audit_url"`
	EnableHTTPS     *bool  `json:"enable_https"`
}

// loadFileConfig reads and parses a JSON config file.
func loadFileConfig(path string) (fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, err
	}
	var fc fileConfig
	if err = json.Unmarshal(data, &fc); err != nil {
		return fileConfig{}, err
	}
	return fc, nil
}

// Load reads configuration in priority order: env vars > flags > JSON file > defaults.
// It must be called once at program startup, before any flags are parsed elsewhere.
func Load() (*Config, error) {
	cfg := &Config{}

	env.Parse(cfg)

	envServerAddress := cfg.ServerAddress
	envBaseURL := cfg.BaseURL
	envFileStoragePath := cfg.FileStoragePath
	envDatabaseDSN := cfg.DatabaseDSN
	envAuditFile := cfg.AuditFile
	envAuditURL := cfg.AuditURL
	envEnableHTTPS := cfg.EnableHTTPS

	envConfigFile := os.Getenv("CONFIG")
	var flagConfigFile string

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for shortened URLs")
	flag.StringVar(&cfg.FileStoragePath, "f", "/tmp/shortener-db.json", "File storage path")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "Database connection string (DSN)")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "Path to audit log file (disabled if empty)")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "URL of remote audit server (disabled if empty)")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "Enable HTTPS with a self-signed certificate")
	flag.StringVar(&flagConfigFile, "c", "", "Path to JSON config file")
	flag.StringVar(&flagConfigFile, "config", "", "Path to JSON config file")

	flag.Parse()

	explicit := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	configPath := envConfigFile
	if configPath == "" {
		configPath = flagConfigFile
	}
	var fc fileConfig
	if configPath != "" {
		var err error
		fc, err = loadFileConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("config file %q: %w", configPath, err)
		}
	}

	if envServerAddress != "" {
		cfg.ServerAddress = envServerAddress
	} else if !explicit["a"] && fc.ServerAddress != "" {
		cfg.ServerAddress = fc.ServerAddress
	}

	if envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	} else if !explicit["b"] && fc.BaseURL != "" {
		cfg.BaseURL = fc.BaseURL
	}

	if envFileStoragePath != "" {
		cfg.FileStoragePath = envFileStoragePath
	} else if !explicit["f"] && fc.FileStoragePath != "" {
		cfg.FileStoragePath = fc.FileStoragePath
	}

	if envDatabaseDSN != "" {
		cfg.DatabaseDSN = envDatabaseDSN
	} else if !explicit["d"] && fc.DatabaseDSN != "" {
		cfg.DatabaseDSN = fc.DatabaseDSN
	}

	if envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	} else if !explicit["audit-file"] && fc.AuditFile != "" {
		cfg.AuditFile = fc.AuditFile
	}

	if envAuditURL != "" {
		cfg.AuditURL = envAuditURL
	} else if !explicit["audit-url"] && fc.AuditURL != "" {
		cfg.AuditURL = fc.AuditURL
	}

	// Bool field: *bool in fileConfig distinguishes absent from false.
	if envEnableHTTPS {
		cfg.EnableHTTPS = true
	} else if !explicit["s"] && fc.EnableHTTPS != nil {
		cfg.EnableHTTPS = *fc.EnableHTTPS
	}

	return cfg, nil
}
