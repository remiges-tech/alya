package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx"
	_ "github.com/lib/pq"
	"github.com/remiges-tech/alya/batch/pg"
)

func main() {

	dbHost := "localhost"
	dbPort := 5432
	dbUser := "alyatest"
	dbPassword := "alyatest"
	dbName := "alyatest"

	ctx := context.Background()
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	// connStr := appConfig.DBConnURL
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Fatal("error connecting db")
	}
	defer conn.Close(ctx)
	// sqlc
	querier := pg.NewProvider(connStr)

	// // Database connection
	// connURL := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	// connPool, err := pg.NewProvider(connURL)
	// if err != nil {
	// 	log.Fatalln("Failed to establishes a connection with database", err)
	// }
	// queries := sqlc.New(connPool)
	// slowQuery := batch.SlowQuery{Queries: queries, Db: connPool}
	fmt.Println(slowQuery.Queries) // just to make compiler happy while I'm developing slowquery module

	r := gin.Default()

	// Start the service
	if err := r.Run(":" + "8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
