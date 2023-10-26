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
	loggers := logHarbour.LogInit("app1", "module1", "system1")
	// Create a new ConfigClient
	client := rigelclient.ConfigClient{
		BaseURL: "http://localhost:8080/rigel", // replace with your config service URL
		Client:  resty.New(),
	}

	logHarbour.LogWrite(loggers.ActivityLogger, logHarbour.LevelInfo, "spanid1", "correlationid1", time.Now().AddDate(1, 0, 0), "bhavya", "127.0.0.1",
		"newLog", "valueBeingUpdated", "id1", 1, "This is an activity logger info message", "somekey", "somevalue", "key2", "value2")

	configName := "C3"
	schemaName := "S1"

	// Create an instance of your application's config struct
	var appConfig infra.AppConfig

	// Call the LoadConfig function
	err := client.LoadConfig(configName, schemaName, &appConfig)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

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
