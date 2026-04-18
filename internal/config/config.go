// Package config loads and validates environment-based configuration
// for the koku-cost-provider service.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	env "github.com/caarlos0/env/v11"
)

type ServerConfig struct {
	BindAddress     string        `env:"SP_SERVER_ADDRESS"          envDefault:":8080"`
	ShutdownTimeout time.Duration `env:"SP_SERVER_SHUTDOWN_TIMEOUT" envDefault:"15s"`
	RequestTimeout  time.Duration `env:"SP_SERVER_REQUEST_TIMEOUT"  envDefault:"30s"`
	ReadTimeout     time.Duration `env:"SP_SERVER_READ_TIMEOUT"     envDefault:"15s"`
	WriteTimeout    time.Duration `env:"SP_SERVER_WRITE_TIMEOUT"    envDefault:"15s"`
	IdleTimeout     time.Duration `env:"SP_SERVER_IDLE_TIMEOUT"     envDefault:"60s"`
	MaxBodySize     int64         `env:"SP_SERVER_MAX_BODY_SIZE"    envDefault:"1048576"` // 1 MB
}

type RegistrationConfig struct {
	DCMURL         string        `env:"DCM_REGISTRATION_URL,required"`
	ProviderName   string        `env:"SP_NAME"              envDefault:"koku-cost-provider"`
	Endpoint       string        `env:"SP_ENDPOINT,required"`
	InitialBackoff time.Duration `env:"SP_REGISTRATION_INITIAL_BACKOFF" envDefault:"1s"`
	MaxBackoff     time.Duration `env:"SP_REGISTRATION_MAX_BACKOFF"     envDefault:"5m"`
	DisplayName    string        `env:"SP_DISPLAY_NAME"      envDefault:""`
}

type KokuConfig struct {
	APIURL       string `env:"KOKU_API_URL,required"`
	Identity     string `env:"KOKU_IDENTITY"          envDefault:""`
	IdentityFile string `env:"KOKU_IDENTITY_FILE"     envDefault:""`
}

type NATSConfig struct {
	URL string `env:"SP_NATS_URL,required"`
}

type StoreConfig struct {
	DBPath string `env:"SP_DB_PATH" envDefault:"data/cost-provider.db"`
}

type ReconcilerConfig struct {
	PollInterval     time.Duration `env:"SP_RECONCILER_POLL_INTERVAL"     envDefault:"5m"`
	ProvisionTimeout time.Duration `env:"SP_RECONCILER_PROVISION_TIMEOUT" envDefault:"24h"`
}

type Config struct {
	Server       ServerConfig
	Registration RegistrationConfig
	Koku         KokuConfig
	NATS         NATSConfig
	Store        StoreConfig
	Reconciler   ReconcilerConfig
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing config from env: %w", err)
	}

	if err := resolveKokuIdentity(&cfg.Koku); err != nil {
		return nil, err
	}

	return cfg, nil
}

// resolveKokuIdentity loads the Koku identity from a file when
// KOKU_IDENTITY_FILE is set, preferring the file over the env var.
func resolveKokuIdentity(koku *KokuConfig) error {
	if koku.IdentityFile != "" {
		data, err := os.ReadFile(koku.IdentityFile)
		if err != nil {
			return fmt.Errorf("reading KOKU_IDENTITY_FILE (%s): %w", koku.IdentityFile, err)
		}
		koku.Identity = strings.TrimSpace(string(data))
	}
	if koku.Identity == "" {
		return fmt.Errorf("one of KOKU_IDENTITY or KOKU_IDENTITY_FILE must be set")
	}
	return nil
}

// RedactedIdentity returns a masked representation safe for logging.
func (k *KokuConfig) RedactedIdentity() string {
	if len(k.Identity) <= 8 {
		return "***"
	}
	return k.Identity[:4] + "..." + k.Identity[len(k.Identity)-4:]
}
