package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/remiges-tech/alya/config"
	"github.com/remiges-tech/alya/examples/pg"
	usersvc "github.com/remiges-tech/alya/examples/userservice"
	"github.com/remiges-tech/alya/logger"
	"github.com/remiges-tech/alya/router"
	"github.com/remiges-tech/alya/service"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
	// Import the rigel client library
)

type AppConfig struct {
	DBConnURL        string `json:"db_conn_url"`
	DBHost           string `json:"db_host"`
	DBPort           int    `json:"db_port"`
	DBUser           string `json:"db_user"`
	DBPassword       string `json:"db_password"`
	DBName           string `json:"db_name"`
	AppServerPort    string `json:"app_server_port"`
	KeycloakURL      string `json:"keycloak_url"`
	KeycloakClientID string `json:"keycloak_client_id"`
}

func main() {
	configSystem := flag.String("configSource", "file", "The configuration system to use (file or rigel)")
	configFilePath := flag.String("configFile", "./config.json", "The path to the configuration file")
	rigelConfigName := flag.String("configName", "C1", "The name of the configuration")
	rigelSchemaName := flag.String("schemaName", "S1", "The name of the schema")
	etcdEndpoints := flag.String("etcdEndpoints", "localhost:2379", "Comma-separated list of etcd endpoints")

	flag.Parse()

	var appConfig AppConfig
	switch *configSystem {
	case "file":
		err := config.LoadConfigFromFile(*configFilePath, &appConfig)
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
	case "rigel":
		err := config.LoadConfigFromRigel(*etcdEndpoints, *rigelConfigName, *rigelSchemaName, &appConfig)
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
	default:
		log.Fatalf("Unknown configuration system: %s", *configSystem)
	}

	fmt.Printf("Loaded configuration: %+v\n", appConfig)

	// Open the error types file
	file, err := os.Open("./errortypes.yaml")
	if err != nil {
		log.Fatalf("Failed to open error types file: %v", err)
	}
	defer file.Close()

	// Load the error types
	wscutils.LoadErrorTypes(file)

	// logger
	fallbackWriter := logharbour.NewFallbackWriter(os.Stdout, os.Stdout)
	lh := logharbour.NewLogger("MyApp", fallbackWriter)
	lh.WithPriority(logharbour.Debug2)
	fl := logger.NewFileLogger("/tmp/idshield.log")

	// auth middleware

	cache := router.NewRedisTokenCache("localhost:6379", "", 0, 0)
	authMiddleware, err := router.LoadAuthMiddleware("alyatest", "https://lemur-7.cloud-iam.com/auth/realms/cool5", cache, fl)
	if err != nil {
		log.Fatalf("Failed to create new auth middleware: %v", err)
	}

	// router

	r, err := router.SetupRouter(true, fl, authMiddleware)
	if err != nil {
		log.Fatalf("Failed to setup router: %v", err)
	}

	// Logging middleware
	r.Use(func(c *gin.Context) {
		log.Printf("[request] %s - %s %s\n", c.Request.RemoteAddr, c.Request.Method, c.Request.URL.Path)
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		log.Printf("[request] %s - %s %s %s\n", c.Request.RemoteAddr, c.Request.Method, c.Request.URL.Path, duration)
	})

	// Create a new service for /hello
	helloService := service.NewService(r).WithLogHarbour(lh)

	// Define the handler function
	handleHelloRequest := func(c *gin.Context, s *service.Service) {
		// Use s.Logger.Log to log a message
		helloService.LogHarbour.LogActivity("Processing /hello request", nil)

		// Process the request
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello, World!",
		})
	}

	// Register routes
	helloService.RegisterRoute(http.MethodGet, "/hello", handleHelloRequest)

	// Create a new service for /users
	userService := service.NewService(r).WithLogHarbour(lh)

	// set up database connection
	pgConfig := pg.Config{
		Host:     appConfig.DBHost,
		Port:     appConfig.DBPort,
		User:     appConfig.DBUser,
		Password: appConfig.DBPassword,
		DBName:   appConfig.DBName,
	}
	queries := pg.NewProvider(pgConfig).Queries()
	userService.WithDatabase(queries)
	userService.RegisterRoute(http.MethodPost, "/users", usersvc.HandleCreateUserRequest)

	// Start the service
	if err := r.Run(":" + appConfig.AppServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
