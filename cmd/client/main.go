package main

import (
	"fmt"
	"go-framework/pkg/rigelclient"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"go-framework/internal/infra"
	"go-framework/internal/webservices/rigel"
	"go-framework/internal/webservices/user"
	voucher "go-framework/internal/webservices/vouchers"
	_ "go-framework/pkg/rigelclient"
)

// Define your application's config struct
type AppConfig struct {
	DatabaseURL string `json:"database_url"`
	Port        int    `json:"port"`
}

func main() {
	//sqlq, lh, rdb := infra.InitInfraServices()
	sqlq, lh, _ := infra.InitInfraServices()
	//r := infra.SetupRouter(lh, rdb)
	r := gin.Default()

	// Pass the Env to the handler functions to interact with database
	voucherHandler := voucher.NewHandler(sqlq, lh)
	userHandler := user.NewHandler(sqlq, lh)
	rigelHandler := rigel.NewHandler(sqlq, lh)

	// Register api handlers
	voucherHandler.RegisterVoucherHandlers(r)
	userHandler.RegisterUserHandlers(r)
	rigelHandler.RegisterHandlers(r)

	// Create a new ConfigClient
	client := rigelclient.ConfigClient{
		BaseURL: "http://localhost:8080/rigel", // replace with your config service URL
		Client:  resty.New(),
	}

	configName := "C3"
	schemaName := "S1"

	// Create an instance of your application's config struct
	var appConfig AppConfig

	// Call the LoadConfig function
	err := client.LoadConfig(configName, schemaName, &appConfig)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	fmt.Printf("App Config: %+v\n", appConfig)
}
