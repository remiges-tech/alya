package db

import (
	"database/sql"
	"fmt"
	"go-framework/internal/pg/sqlc-gen"
	"log"

	_ "github.com/lib/pq" // PostgreSQL driver
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

type Provider struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewProvider(cfg Config) *Provider {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Successfully connected to the database")

	queries := sqlc.New(db)

	return &Provider{db: db, queries: queries}
}

func (p *Provider) DB() *sql.DB {
	return p.db
}

func (p *Provider) Queries() *sqlc.Queries {
	return p.queries
}
