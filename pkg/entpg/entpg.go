package entpg

import (
	"database/sql"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func OpenDriver(pool *pgxpool.Pool) (dialect.Driver, *sql.DB) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	return entsql.OpenDB(dialect.Postgres, sqlDB), sqlDB
}
