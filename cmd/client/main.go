package main

import (
	"fmt"
	"go-framework/pkg/rigelclient"
	"log"

	"github.com/go-resty/resty/v2"
	_ "go-framework/pkg/rigelclient"
)

// Define your application's config struct
type AppConfig struct {
	DatabaseURL string `json:"database_url"`
	Port        int    `json:"port"`
}

func main() {

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
