package config

import (
	"time"

	configpkg "github.com/rajabinekoo/sigryx/pkg/config"
)

type Config struct {
	ServiceName string `env:"SERVICE_NAME" envDefault:"sigryx"`

	// Logging
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// Postgres
	PostgresDSN             string        `env:"POSTGRES_DSN,required"`
	PostgresSchema          string        `env:"POSTGRES_SCHEMA" envDefault:"vault"`
	PostgresMaxConns        int32         `env:"POSTGRES_MAX_CONNS" envDefault:"10"`
	PostgresMinConns        int32         `env:"POSTGRES_MIN_CONNS" envDefault:"2"`
	PostgresConnectTimeout  time.Duration `env:"POSTGRES_CONNECT_TIMEOUT" envDefault:"5s"`
	PostgresMaxConnLifetime time.Duration `env:"POSTGRES_MAX_CONN_LIFETIME" envDefault:"30m"`
	PostgresMaxConnIdleTime time.Duration `env:"POSTGRES_MAX_CONN_IDLE_TIME" envDefault:"5m"`

	// http
	HTTPAddr            string        `env:"HTTP_ADDR" envDefault:":8080"`
	HTTPReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	HTTPWriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"15s"`
	HTTPIdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	HTTPShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`

	// Authentication
	SetupToken        string        `env:"SETUP_TOKEN"`
	AuthJWTSecret     string        `env:"AUTH_JWT_SECRET,required"`
	AuthIssuer        string        `env:"AUTH_ISSUER" envDefault:"sigryx"`
	AuthAudience      string        `env:"AUTH_AUDIENCE" envDefault:"sigryx-api"`
	AuthAccessTTL     time.Duration `env:"AUTH_ACCESS_TTL" envDefault:"10m"`
	AuthRefreshTTL    time.Duration `env:"AUTH_REFRESH_TTL" envDefault:"168h"`
	TrustedProxyCIDRs string        `env:"TRUSTED_PROXY_CIDRS"`

	// Audit retention
	AuditNormalRetentionDays   int           `env:"AUDIT_NORMAL_RETENTION_DAYS" envDefault:"30"`
	AuditCriticalRetentionDays int           `env:"AUDIT_CRITICAL_RETENTION_DAYS" envDefault:"365"`
	AuditCleanupInterval       time.Duration `env:"AUDIT_CLEANUP_INTERVAL" envDefault:"6h"`
	AuditCleanupBatchSize      int           `env:"AUDIT_CLEANUP_BATCH_SIZE" envDefault:"5000"`

	// Integrity signing / alerting
	AlertWebhookURL     string        `env:"ALERT_WEBHOOK_URL"`
	AlertWebhookTimeout time.Duration `env:"ALERT_WEBHOOK_TIMEOUT" envDefault:"2s"`

	// App Envs
	MaxUnsealSize int `env:"MAX_UNSEAL_SIZE" envDefault:"10"`
}

func Load() (Config, error) {
	var cfg Config
	if err := configpkg.Load(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
