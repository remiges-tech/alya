// Package config provides startup loading and runtime reads for Alya services.
//
// The package separates two jobs:
//
//   - Loader reads startup configuration into a struct.
//   - Provider reads individual values and can watch for updates.
//
// Most applications start with one of these loaders:
//
//   - NewFile(path) for JSON files
//   - NewEnv(prefix) for environment variables
//   - NewRigel(client) for Rigel-backed config
//
// Common startup flow:
//
//	loader, err := config.NewFile("config.json")
//	if err != nil {
//	    return err
//	}
//
//	var cfg AppConfig
//	if err := config.LoadWith(loader, &cfg); err != nil {
//	    return err
//	}
//
// File and Rigel values can also be read through the Provider interface with Get,
// GetInt, GetBool, and Watch.
//
// See README.md in this directory for end-to-end examples.
package config
