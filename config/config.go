package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ConfigSource is an interface that represents a source from which application configuration can be loaded.
type ConfigSource interface {
	// LoadConfig reads config from the source and binds it to c.
	LoadConfig(c any) error
	// Check checks if the config source can be used. For example, a file config source would check
	// if the file exists. A Rigel config source would check if the Rigel server is available.
	Check() error
}

// Load first ensures that the config system valid and accessible. Then it loads the config into c.
func Load(cs ConfigSource, c any) error {
	if err := cs.Check(); err != nil {
		return err
	}
	return cs.LoadConfig(c)
}

// File

type File struct {
	ConfigFilePath string
}

func (f *File) Check() error {
	if f.ConfigFilePath == "" {
		return fmt.Errorf("configFilePath cannot be empty")
	}

	return nil
}

func (f *File) LoadConfig(appConfig any) error {
	filePath := f.ConfigFilePath
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(appConfig)
}

// Rigel

type Rigel struct {
	ServerURL  string
	ConfigName string
	SchemaName string
}

func (r *Rigel) Check() error {
	if r.ServerURL == "" {
		return fmt.Errorf("ServerURL cannot be empty")
	}

	if r.ConfigName == "" {
		return fmt.Errorf("ConfigName cannot be empty")
	}

	if r.SchemaName == "" {
		return fmt.Errorf("SchemaName cannot be empty")
	}

	return nil
}

func (r *Rigel) LoadConfig(config any) error {
	// use rigel client to load config
	return nil
}
