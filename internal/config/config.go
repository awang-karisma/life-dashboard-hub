package config

import (
	"errors"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	DatabaseURL   string `mapstructure:"DATABASE_URL"`
	DataDir       string `mapstructure:"DATA_DIR"`
	ListenAddr    string `mapstructure:"LISTEN_ADDR"`
	SessionSecret string `mapstructure:"SESSION_SECRET"`
	PublicURL     string `mapstructure:"PUBLIC_URL"`
	TZ            string `mapstructure:"TZ"`
	FrontendMode  string `mapstructure:"FRONTEND_MODE"`
	LogLevel      string `mapstructure:"LOG_LEVEL"`
	RetentionDays int    `mapstructure:"RETENTION_DAYS"`
	MaxBodyBytes  int64  `mapstructure:"MAX_BODY_BYTES"`

	// OIDC/OAuth2 (all optional — when OIDC_ISSUER_URL is set, OIDC login is enabled)
	OIDCIssuerURL    string `mapstructure:"OIDC_ISSUER_URL"`
	OIDCClientID     string `mapstructure:"OIDC_CLIENT_ID"`
	OIDCClientSecret string `mapstructure:"OIDC_CLIENT_SECRET"`
	OIDCScopes       string `mapstructure:"OIDC_SCOPES"`
	OIDCDisplayName  string `mapstructure:"OIDC_DISPLAY_NAME"`
}

func Load() (*Config, error) {
	// Load .env if present; ignore error if missing
	_ = godotenv.Load()

	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.life-dashboard-hub")
	v.AddConfigPath("/etc/life-dashboard-hub")

	// Set defaults
	v.SetDefault("DATA_DIR", "./data")
	v.SetDefault("LISTEN_ADDR", ":8080")
	v.SetDefault("TZ", "UTC")
	v.SetDefault("FRONTEND_MODE", "embed")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("RETENTION_DAYS", 365)
	v.SetDefault("MAX_BODY_BYTES", int64(32*1024*1024))
	v.SetDefault("OIDC_SCOPES", "openid email profile")
	v.SetDefault("OIDC_DISPLAY_NAME", "SSO")

	// Explicitly bind environment variables so Unmarshal captures them reliably
	keys := []string{
		"DATABASE_URL",
		"DATA_DIR",
		"LISTEN_ADDR",
		"SESSION_SECRET",
		"PUBLIC_URL",
		"TZ",
		"FRONTEND_MODE",
		"LOG_LEVEL",
		"RETENTION_DAYS",
		"MAX_BODY_BYTES",
		"OIDC_ISSUER_URL",
		"OIDC_CLIENT_ID",
		"OIDC_CLIENT_SECRET",
		"OIDC_SCOPES",
		"OIDC_DISPLAY_NAME",
	}
	for _, k := range keys {
		_ = v.BindEnv(k)
	}

	// Read in config file if present; ignore file-not-found error
	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Validate required fields
	if cfg.SessionSecret == "" {
		return nil, errors.New("SESSION_SECRET is required")
	}

	return &cfg, nil
}
