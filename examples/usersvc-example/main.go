package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/examples/usersvc-example/pg"
	usersvc "github.com/remiges-tech/alya/examples/usersvc-example/userservice"
	"github.com/remiges-tech/alya/service"
	"github.com/remiges-tech/logharbour/logharbour"

	"github.com/remiges-tech/rigel"
	"github.com/remiges-tech/rigel/etcd"
)

func main() {
	// Initialize etcd storage for Rigel
	etcdEndpoints := []string{"localhost:2379"}
	etcdStorage, err := etcd.NewEtcdStorage(etcdEndpoints)
	if err != nil {
		log.Fatalf("Failed to create EtcdStorage: %v", err)
	}

	// Initialize Rigel client
	rigelClient := rigel.New(etcdStorage, "alya", "usersvc", 1, "dev")

	// Create context
	ctx := context.Background()

	// Initialize logger with proper components
	fallbackWriter := logharbour.NewFallbackWriter(os.Stdout, os.Stdout)
	lctx := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(lctx, "UserService", fallbackWriter)
	logger.WithPriority(logharbour.Debug2)

	// Get database configuration dynamically
	host, err := rigelClient.Get(ctx, "database.host")
	if err != nil {
		log.Fatalf("Failed to get database host: %v", err)
	}
	port, err := rigelClient.GetInt(ctx, "database.port")
	if err != nil {
		log.Fatalf("Failed to get database port: %v", err)
	}
	user, err := rigelClient.Get(ctx, "database.user")
	if err != nil {
		log.Fatalf("Failed to get database user: %v", err)
	}
	password, err := rigelClient.Get(ctx, "database.password")
	if err != nil {
		log.Fatalf("Failed to get database password: %v", err)
	}
	dbname, err := rigelClient.Get(ctx, "database.dbname")
	if err != nil {
		log.Fatalf("Failed to get database name: %v", err)
	}

	// Initialize database
	dbConfig := pg.Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbname,
	}
	provider := pg.NewProvider(dbConfig)
	db := provider.Queries()

	// Create Gin router
	router := gin.Default()

	// Create service with Rigel client
	s := service.NewService(router).
		WithLogHarbour(logger).
		WithDatabase(db).
		WithRigelConfig(rigelClient)

	// Register routes
	s.RegisterRoute("POST", "/users", usersvc.HandleCreateUserRequest)

	// Get server port dynamically
	serverPort, err := rigelClient.GetInt(ctx, "server.port")
	if err != nil {
		log.Fatalf("Failed to get server port: %v", err)
	}

	// Start server
	serverAddr := fmt.Sprintf(":%d", serverPort)
	if err := router.Run(serverAddr); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
