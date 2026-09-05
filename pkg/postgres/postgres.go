package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const DefaultSchema = "vault"

var schemaNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

type Config struct {
	DSN             string        `env:"POSTGRES_DSN,required"`
	Schema          string        `env:"POSTGRES_SCHEMA" envDefault:"vault"`
	MaxConns        int32         `env:"POSTGRES_MAX_CONNS" envDefault:"10"`
	MinConns        int32         `env:"POSTGRES_MIN_CONNS" envDefault:"2"`
	MaxConnLifetime time.Duration `env:"POSTGRES_MAX_CONN_LIFETIME" envDefault:"30m"`
	MaxConnIdleTime time.Duration `env:"POSTGRES_MAX_CONN_IDLE_TIME" envDefault:"5m"`
	ConnectTimeout  time.Duration `env:"POSTGRES_CONNECT_TIMEOUT" envDefault:"5s"`
}

func ValidateSchemaName(schema string) error {
	schema = strings.TrimSpace(schema)
	if !schemaNamePattern.MatchString(schema) {
		return fmt.Errorf("postgres: invalid schema %q: must match %s", schema, schemaNamePattern.String())
	}
	if schema == "information_schema" || strings.HasPrefix(schema, "pg_") {
		return fmt.Errorf("postgres: schema %q is reserved", schema)
	}
	return nil
}

func New(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.Schema == "" {
		cfg.Schema = DefaultSchema
	}
	if err := ValidateSchemaName(cfg.Schema); err != nil {
		return nil, err
	}

	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse dsn: %w", err)
	}
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MinConns = cfg.MinConns
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	// Never fall back to public for application queries. Keeping pg_catalog in
	// the path preserves access to PostgreSQL built-ins while all unqualified
	// Sigryx tables/functions resolve only inside the configured app schema.
	if pcfg.ConnConfig.RuntimeParams == nil {
		pcfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	pcfg.ConnConfig.RuntimeParams["search_path"] = cfg.Schema + ",pg_catalog"

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(pingCtx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: open pool: %w", err)
	}
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

func OpenSQL(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: open sql db: %w", err)
	}

	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: ping sql db: %w", err)
	}

	return db, nil
}

func Checker(pool *pgxpool.Pool) func(ctx context.Context) error {
	return func(ctx context.Context) error { return pool.Ping(ctx) }
}
