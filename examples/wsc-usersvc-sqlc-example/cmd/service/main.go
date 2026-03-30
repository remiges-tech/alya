package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/config"
	pg "github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/db"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/http"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/repository"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/service"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/validation"
	"github.com/remiges-tech/logharbour/logharbour"
)

// AppConfig holds the startup configuration.
type AppConfig struct {
	Database struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
	} `json:"database"`
	Server struct {
		Port int `json:"port"`
	} `json:"server"`
}

type userOrderRepository interface {
	repository.UserRepository
	repository.OrderRepository
}

func main() {
	fallbackWriter := logharbour.NewFallbackWriter(os.Stdout, os.Stdout)
	loggerContext := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(loggerContext, "WSCUserService", fallbackWriter)
	logger.WithPriority(logharbour.Debug2)

	var cfg AppConfig
	configSource, sourceLabel, err := newConfigLoader()
	if err != nil {
		logger.Error(fmt.Errorf("failed to create config loader: %w", err)).LogActivity("Configuration error", nil)
		os.Exit(1)
	}
	if err := config.LoadWith(configSource, &cfg); err != nil {
		logger.Error(fmt.Errorf("failed to load startup config: %w", err)).LogActivity("Configuration error", map[string]any{"source": sourceLabel})
		os.Exit(1)
	}
	logger.Info().LogActivity("Configuration loaded", map[string]any{"source": sourceLabel})

	providerCfg := pg.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
	}

	backend := strings.ToLower(strings.TrimSpace(os.Getenv("ALYA_REPOSITORY_BACKEND")))
	repo, backendLabel := newRepository(providerCfg, backend)
	logger.Info().LogActivity("Database connection initialized", map[string]any{"backend": backendLabel, "host": providerCfg.Host, "port": providerCfg.Port, "db": providerCfg.DBName})

	userAppService := app.NewUserService(repo)
	orderAppService := app.NewOrderService(repo, repo)
	userValidator := validation.NewUserValidator()
	orderValidator := validation.NewOrderValidator()
	userHandler := transport.NewUserHandler(userAppService, userValidator, logger.WithModule("UserHandler"))
	orderHandler := transport.NewOrderHandler(orderAppService, orderValidator, logger.WithModule("OrderHandler"))

	router := gin.Default()
	transport.RegisterRoutes(router, userHandler, orderHandler)

	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	logger.Info().LogActivity("Starting server", map[string]any{"addr": serverAddr})
	if err := router.Run(serverAddr); err != nil {
		logger.Error(fmt.Errorf("error starting server: %w", err)).LogActivity("Server startup failed", nil)
		os.Exit(1)
	}
}

func newConfigLoader() (config.Loader, string, error) {
	source := strings.ToLower(strings.TrimSpace(os.Getenv("CONFIG_SOURCE")))
	switch source {
	case "", "file":
		configFilePath := getConfigFilePath()
		loader, err := config.NewFile(configFilePath)
		if err != nil {
			return nil, "", err
		}
		return loader, fmt.Sprintf("file:%s", configFilePath), nil
	case "env":
		return config.NewEnv(getEnvPrefix()), fmt.Sprintf("env:%s", getEnvPrefix()), nil
	default:
		return nil, "", fmt.Errorf("unsupported CONFIG_SOURCE: %s", source)
	}
}

func getConfigFilePath() string {
	if path := os.Getenv("CONFIG_FILE"); path != "" {
		return path
	}
	return "config.json"
}

func getEnvPrefix() string {
	return "ALYA_WSC_USERSVC"
}

func newRepository(cfg pg.Config, backend string) (userOrderRepository, string) {
	switch backend {
	case "", "sqlc":
		provider := pg.NewProvider(cfg)
		return repository.NewSQLCRepository(provider.Queries()), "sqlc"
	case "gorm":
		provider := pg.NewGormProvider(cfg)
		return repository.NewGORMRepository(provider.DB()), "gorm"
	default:
		fmt.Fprintf(os.Stderr, "unsupported repository backend: %s\n", backend)
		os.Exit(1)
		return nil, ""
	}
}
