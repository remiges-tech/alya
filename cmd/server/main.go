package main

import (
	"go-framework/internal/infra"
	"go-framework/internal/webservices/rigel"
	"go-framework/pkg/rigelclient"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/remiges-tech/logharbour/logHarbour"
)

func main() {
	loggers := logHarbour.LogInit("rigel", "main", "system1")
	// Create a new ConfigClient
	client := rigelclient.ConfigClient{
		BaseURL: "http://localhost:8080/rigel", // replace with your config service URL
		Client:  resty.New(),
	}

	logHarbour.LogWrite(loggers.ActivityLogger, logHarbour.LevelInfo, "na", "na", time.Now(), "na", "na",
		"damonStart", "na", "na", 1, "Rigel server started")

	configName := "C3"
	schemaName := "S1"

	// Create an instance of your application's config struct
	var appConfig infra.AppConfig

	// Call the LoadConfig function
	err := client.LoadConfig(configName, schemaName, &appConfig)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	logHarbour.LogWrite(loggers.ActivityLogger, logHarbour.LevelInfo, "na", "na", time.Now(), "na", "na",
		"load_appConfig", "na", "na", 1, "Appconfig loaded", "db_host", appConfig.DBHost)

	//sqlq, lh, rdb := infra.InitInfraServices()
	dbProvider, lh, _ := infra.InitInfraServices(appConfig)
	sqlq := dbProvider.Queries()
	r := gin.Default()

	// Pass the Env to the handler functions to interact with database
	rigelHandler := rigel.NewHandler(sqlq, lh)

	// Register api handlers
	rigelHandler.RegisterHandlers(r)

	r.Run(":8080")
}
