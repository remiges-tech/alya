package main

import (
	"context"
	"fmt"
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
	// Initialize logger with proper components
	fallbackWriter := logharbour.NewFallbackWriter(os.Stdout, os.Stdout)
	lctx := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(lctx, "UserService", fallbackWriter)
	logger.WithPriority(logharbour.Debug2)

	// Initialize etcd storage for Rigel
	etcdEndpoints := []string{"localhost:2379"}
	etcdStorage, err := etcd.NewEtcdStorage(etcdEndpoints)
	if err != nil {
		logger.Error(fmt.Errorf("failed to create EtcdStorage: %w", err)).LogActivity("Startup failed", nil)
		os.Exit(1)
	}

	// Initialize Rigel client
	rigelClient := rigel.New(etcdStorage, "alya", "usersvc", 1, "dev")
	logger.Info().LogActivity("Rigel client initialized", nil)

	// Create context
	ctx := context.Background()

	// Get database configuration dynamically
	host, err := rigelClient.Get(ctx, "database.host")
	if err != nil {
		logger.Error(fmt.Errorf("failed to get database host: %w", err)).LogActivity("Configuration error", nil)
		os.Exit(1)
	}
	port, err := rigelClient.GetInt(ctx, "database.port")
	if err != nil {
		logger.Error(fmt.Errorf("failed to get database port: %w", err)).LogActivity("Configuration error", nil)
		os.Exit(1)
	}
	user, err := rigelClient.Get(ctx, "database.user")
	if err != nil {
		logger.Error(fmt.Errorf("failed to get database user: %w", err)).LogActivity("Configuration error", nil)
		os.Exit(1)
	}
	password, err := rigelClient.Get(ctx, "database.password")
	if err != nil {
		logger.Error(fmt.Errorf("failed to get database password: %w", err)).LogActivity("Configuration error", nil)
		os.Exit(1)
	}
	dbname, err := rigelClient.Get(ctx, "database.dbname")
	if err != nil {
		logger.Error(fmt.Errorf("failed to get database name: %w", err)).LogActivity("Configuration error", nil)
		os.Exit(1)
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
	logger.Info().LogActivity("Database connection initialized", map[string]any{
		"host": host,
		"port": port,
		"user": user,
		"db":   dbname,
	})

	// Create Gin router
	router := gin.Default()

	// Create service with Rigel client
	s := service.NewService(router).
		WithLogHarbour(logger).
		WithDatabase(db).
		WithRigelConfig(rigelClient)

	// Register routes
	s.RegisterRoute("POST", "/users", usersvc.HandleCreateUserRequest)
	logger.Info().LogActivity("Routes registered", nil)

	// Get server port dynamically
	serverPort, err := rigelClient.GetInt(ctx, "server.port")
	if err != nil {
		logger.Error(fmt.Errorf("failed to get server port: %w", err)).LogActivity("Configuration error", nil)
		os.Exit(1)
	}

	// Start server
	serverAddr := fmt.Sprintf(":%d", serverPort)
	logger.Info().LogActivity("Starting server", map[string]any{"port": serverPort})
	if err := router.Run(serverAddr); err != nil {
		logger.Error(fmt.Errorf("error starting server: %w", err)).LogActivity("Server startup failed", nil)
		os.Exit(1)
	}
}
