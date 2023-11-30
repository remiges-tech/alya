package config

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/remiges-tech/rigel" // Import the rigel client library
	"github.com/remiges-tech/rigel/etcd"
)

func LoadConfigFromFile(filePath string, appConfig any) error {
	configSource, err := NewFile(filePath)
	if err != nil {
		log.Fatalf("Failed to create File config source: %v", err)
	}

	err = Load(configSource, appConfig)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	return nil
}

func LoadConfigFromRigel(etcdEndpoints, configName, schemaName string, appConfig any) error {
	// Create a new EtcdStorage instance
	etcdStorage, err := etcd.NewEtcdStorage(strings.Split(etcdEndpoints, ","))
	if err != nil {
		log.Fatalf("Failed to create EtcdStorage: %v", err)
	}

	// Create a new Rigel instance
	rigelClient := rigel.New(etcdStorage)

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Load the config
	var configSource Config
	err = rigelClient.LoadConfig(ctx, configName, 1, schemaName, &configSource)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	err = Load(configSource, appConfig)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	return nil
}
