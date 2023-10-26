package pg

import (
	"database/sql"
	"fmt"
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
	db *sql.DB
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

	return &Provider{db: db}
}

func (p *Provider) DB() *sql.DB {
	return p.db
}
