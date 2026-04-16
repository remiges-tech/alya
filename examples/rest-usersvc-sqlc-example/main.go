package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/config"
	"github.com/remiges-tech/alya/restutils"
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

func main() {
	var cfg AppConfig
	configSource, sourceLabel, err := newConfigLoader()
	if err != nil {
		log.Fatalf("failed to create config loader: %v", err)
	}
	if err := config.LoadWith(configSource, &cfg); err != nil {
		log.Fatalf("failed to load startup config from %s: %v", sourceLabel, err)
	}

	router := gin.Default()
	_, cleanup, err := Build(router, cfg)
	if err != nil {
		log.Fatalf("failed to build application graph: %v", err)
	}
	defer cleanup()

	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("REST SQLC example listening on %s using %s", serverAddr, sourceLabel)
	if err := router.Run(serverAddr); err != nil {
		log.Fatal(err)
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
	return "ALYA_REST_USERSVC"
}

func newValidator() *restutils.Validator {
	return restutils.NewValidatorWithConfig(restutils.ValidatorConfig{
		FieldRules: map[string]map[string]restutils.ValidationRule{
			"username": {
				"max": {
					MsgID:   7,
					ErrCode: "toobig",
				},
			},
		},
	})
}
