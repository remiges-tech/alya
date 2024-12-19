package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/remiges-tech/rigel" // Import the rigel client library
	"github.com/remiges-tech/rigel/etcd"
)

func LoadConfigFromFile(filePath string, appConfig any) error {
	configSource, err := newFile(filePath)
	if err != nil {
		return fmt.Errorf("Failed to create File config source: %v", err)
	}

	err = Load(configSource, appConfig)
	if err != nil {
		return fmt.Errorf("Error loading config: %v", err)
	}

	return nil
}

func LoadConfigFromRigel(etcdEndpoints, configName, schemaName string, appConfig any) error {
	// Create a new EtcdStorage instance
	etcdStorage, err := etcd.NewEtcdStorage(strings.Split(etcdEndpoints, ","))
	if err != nil {
		return fmt.Errorf("Failed to create EtcdStorage: %v", err)
	}

	// Create a new Rigel instance
	rigelClient := rigel.NewWithStorage(etcdStorage)

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Load the config
	var configSource Config
	err = rigelClient.LoadConfig(ctx, &configSource)
	if err != nil {
		return fmt.Errorf("Failed to load config: %v", err)
	}

	err = Load(configSource, appConfig)
	if err != nil {
		return fmt.Errorf("Error loading config: %v", err)
	}

	return nil
}
