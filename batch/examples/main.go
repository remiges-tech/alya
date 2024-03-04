package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/remiges-tech/alya/batch"
	"github.com/remiges-tech/alya/batch/pg/sqlc"
)

func main() {

	dbHost := "localhost"
	dbPort := 5432
	dbUser := "alyatest"
	dbPassword := "alyatest"
	dbName := "alyatest"

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("error connecting db")
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	// Initialize SlowQuery
	slowQuery := batch.SlowQuery{
		Db:      pool,
		Queries: queries,
	}
	fmt.Println(slowQuery.Queries) // just to make compiler happy while I'm developing slowquery module

	r := gin.Default()

	// Start the service
	if err := r.Run(":" + "8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
