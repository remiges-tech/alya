package pg

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch/pg/sqlc"
)

func NewProvider(connString string) sqlc.Querier {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, connString)
	if err != nil {
		log.Fatal("error connecting db")
	}
	err = db.Ping(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Successfully connected to the database")
	return sqlc.NewQuerierWithTX(db)
}
