package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"go-framework/internal/infra"
	"go-framework/internal/webservices/rigel"
	"go-framework/internal/webservices/user"
	voucher "go-framework/internal/webservices/vouchers"
	"go-framework/pkg/rigelclient"
	"log"
)

func main() {
	// Create a new ConfigClient
	client := rigelclient.ConfigClient{
		BaseURL: "http://localhost:8080/rigel", // replace with your config service URL
		Client:  resty.New(),
	}

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
	voucherHandler := voucher.NewHandler(sqlq, lh)
	userHandler := user.NewHandler(sqlq, lh)
	rigelHandler := rigel.NewHandler(sqlq, lh)

	// Register api handlers
	voucherHandler.RegisterVoucherHandlers(r)
	userHandler.RegisterUserHandlers(r)
	rigelHandler.RegisterHandlers(r)

	r.Run(":8080")
}
