package pg

import (
	"database/sql"
	"fmt"
	"log"

	sqlc "github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/pg/sqlc-gen"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Config holds the database connection settings used by the example provider.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// Provider owns the database handle and the SQLC query wrapper used by the
// example service.
type Provider struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// NewProvider opens the PostgreSQL connection, verifies it with Ping, and builds
// the SQLC query wrapper used by the repository adapter.
func NewProvider(cfg Config) (*Provider, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	log.Println("Successfully connected to the database")
	return &Provider{db: db, queries: sqlc.New(db)}, nil
}

// Close releases the underlying database handle.
func (p *Provider) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

// DB exposes the underlying sql.DB for cases where the example needs raw access.
func (p *Provider) DB() *sql.DB {
	return p.db
}

// Queries exposes the SQLC query wrapper used by the repository adapter.
func (p *Provider) Queries() *sqlc.Queries {
	return p.queries
}
