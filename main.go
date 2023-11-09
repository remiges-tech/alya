package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/config"
	"github.com/remiges-tech/alya/router"
	"log"
	"net/http"
	"time"
)

type AppConfig struct {
	DBConnURL     string `json:"db_conn_url"`
	AppServerPort string `json:"app_server_port"`
}

func main() {
	configSystem := flag.String("configSource", "file", "The configuration system to use (file or rigel)")
	configFilePath := flag.String("configFile", "./config.json", "The path to the configuration file")
	rigelURL := flag.String("rigelURL", "http://localhost:8080/rigel", "The ServerURL of the Rigel server")
	rigelConfigName := flag.String("configName", "C1", "The name of the configuration")
	rigelSchemaName := flag.String("schemaName", "S1", "The name of the schema")
	flag.Parse()

	var options config.ConfigSource
	switch *configSystem {
	case "file":
		options = &config.File{ConfigFilePath: *configFilePath}
	case "rigel":
		options = &config.Rigel{
			ServerURL:  *rigelURL,
			ConfigName: *rigelConfigName,
			SchemaName: *rigelSchemaName,
		}
	default:
		log.Fatalf("Unknown configuration system: %s", *configSystem)
	}

	var appConfig AppConfig
	err := config.Load(options, &appConfig)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	fmt.Printf("Loaded configuration: %+v\n", appConfig)

	// router

	// The developer can choose the specific implementation they want.
	var r router.Router = router.NewGinRouter()

	// Logging middleware
	r.Use(func(ctx router.Context, next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) {
			log.Printf("[request] %s - %s %s\n", c.Request().RemoteAddr, c.Request().Method, c.Request().URL.Path)
			start := time.Now()
			next(c)
			duration := time.Since(start)
			log.Printf("[request] %s - %s %s %s\n", c.Request().RemoteAddr, c.Request().Method, c.Request().URL.Path, duration)
		}
	})

	// Auth middleware
	r.Use(func(ctx router.Context, next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) {
			// For simplicity, let's assume the user is always authenticated.
			log.Println("User authenticated")
			next(c)
		}
	})

	r.GET("/hello", func(c router.Context) {
		c.JSON(http.StatusOK, gin.H{ // todo: gin.H should be replaced with our own type
			"message": "Hello, World!",
		})
	})

	if err := r.Serve(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}
