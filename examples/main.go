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

	// Define a custom validation tag-to-message ID map
	// Custom validation tag-to-error code map
	// Register the custom map with wscutils
	// Set default message ID and error code if needed
	// Set custom message ID and error code for invalid JSON errors
	setupValidation()

	// logger
	fallbackWriter := logharbour.NewFallbackWriter(os.Stdout, os.Stdout)
	lh := logharbour.NewLogger("MyApp", fallbackWriter)
	lh.WithPriority(logharbour.Debug2)
	fl := logger.NewFileLogger("/tmp/idshield.log")

	// auth middleware

	// Customize default message ID and error code
	authMiddleware := setupAuthMiddleware(fl)

	// router
	r, err := router.SetupRouter(true, fl, authMiddleware)
	// r := gin.Default()
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

func setupValidation() {
	customValidationMap := map[string]int{
		"required": 101,
		"email":    102,
		"min":      103,
	}

	customErrCodeMap := map[string]string{
		"required": "required",
		"email":    "invalid_email",
	}

	// SetValidationTagToMsgIDMap updates the mapping of validation tags to specific message IDs.
	// This allows for the customization of error messages based on the type of validation error encountered.
	// The customValidationMap variable should contain a mapping of validation tags (e.g., "required", "email")
	// to their corresponding message IDs.
	wscutils.SetValidationTagToMsgIDMap(customValidationMap)

	// SetValidationTagToErrCodeMap updates the mapping of validation tags to specific error codes.
	// Similar to SetValidationTagToMsgIDMap, this customization enables the application to return
	// more granular and descriptive error codes for different types of validation errors.
	// The customErrCodeMap variable should contain a mapping of validation tags to their corresponding
	// error codes.
	wscutils.SetValidationTagToErrCodeMap(customErrCodeMap)

	// SetDefaultMsgID sets a custom default message ID to be used for validation errors
	// when a specific message ID has not been registered for a validation tag.
	// This ensures that even unregistered validation errors will return a standardized
	// message ID.
	wscutils.SetDefaultMsgID(100)

	// SetDefaultErrCode sets a custom default error code to be used for validation errors
	// when a specific error code has not been registered for a validation tag.
	// This default error code serves as a fallback.
	wscutils.SetDefaultErrCode("validation_err")

	// SetMsgIDInvalidJSON sets a custom message ID for errors related to invalid JSON in requests.
	// This allows the application to return a specific, consistent message ID for such errors.
	wscutils.SetMsgIDInvalidJSON(1000)

	// SetErrCodeInvalidJSON sets a custom error code for errors related to invalid JSON in requests.
	// This complements SetMsgIDInvalidJSON by providing a descriptive error code ("invalid_json")
	wscutils.SetErrCodeInvalidJSON("invalid_json")
}

func setupAuthMiddleware(fl *logger.FileLogger) *router.AuthMiddleware {
	// Set the default message ID for the router's error responses.
	// This ID is used when an error occurs but no specific message ID has been registered for that error scenario.
	router.SetDefaultMsgID(99)

	// Set the default error code for the router's error responses.
	// Similar to the default message ID, this error code is used when an error occurs but no specific error code has been registered for that error scenario.
	router.SetDefaultErrCode("router_error")

	// Register a specific message ID for the TokenMissing error scenario.
	// This msg id is used in the error response indicating that the auth token is missing.
	router.RegisterAuthMsgID(router.TokenMissing, 1001)

	// Register a specific error code for the TokenMissing error scenario.
	// This error code is used in the error response indicating that the auth token is missing.
	router.RegisterAuthErrCode(router.TokenMissing, "token_missing")

	// Register a specific message ID for the TokenCacheFailed error scenario.
	// This message ID is used when there's an issue with the token cache operation, such as failing to retrieve or store a token.
	router.RegisterAuthMsgID(router.TokenCacheFailed, 1002)

	// Register a specific error code for the TokenCacheFailed error scenario.
	// The error code "TOKEN_CACHE_FAILED" is used in the error response to clearly indicate that the error was due to a token cache operation failure.
	router.RegisterAuthErrCode(router.TokenCacheFailed, "token_cache_failed")

	cache := router.NewRedisTokenCache("localhost:6379", "", 0, 0)
	authMiddleware, err := router.LoadAuthMiddleware("alyatest", "https://lemur-7.cloud-iam.com/auth/realms/cool5", cache, fl)
	if err != nil {
		log.Fatalf("Failed to create new auth middleware: %v", err)
	}
	return authMiddleware
}
