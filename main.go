package main

import (
	"flag"
	"fmt"
	"github.com/remiges-tech/alya/config"
	"log"
)

type AppConfig struct {
	DBConnURL     string `json:"db_conn_url"`
	AppServerPort int    `json:"app_server_port"`
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
}
