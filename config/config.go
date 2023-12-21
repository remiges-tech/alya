package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/remiges-tech/rigel"
	"github.com/remiges-tech/rigel/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Config is an interface that represents a source from which application configuration can be loaded.
type Config interface {
	LoadConfig(c any) error
	Check() error
	Get(key string) (string, error)

	// Watch watches for changes to a key in the storage and sends the events to the provided channel.
	// The events includes the key and the updated value.
	// events is the channel to send events when the key's value changes
	Watch(ctx context.Context, key string, events chan<- Event) error
}

// Event represents a change to a key in the storage.
// Key is the key that was changed
// Value is the new value of the key
type Event struct {
	Key   string
	Value string
}

// Load first ensures that the config system valid and accessible. Then it loads the config into c.
func Load(cs Config, c any) error {
	if err := cs.Check(); err != nil {
		return err
	}
	return cs.LoadConfig(c)
}

// File

type File struct {
	ConfigFilePath string
	Config         map[string]interface{}
}

func (f *File) Check() error {
	if f.ConfigFilePath == "" {
		return fmt.Errorf("configFilePath cannot be empty")
	}

	return nil
}

func newFile(configFilePath string) (*File, error) {
	file := &File{ConfigFilePath: configFilePath}

	if err := file.Check(); err != nil {
		return nil, err
	}

	return file, nil
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

type ValueNotStringError struct {
	Key   string
	Value interface{}
}

func (e *ValueNotStringError) Error() string {
	return fmt.Sprintf("value for key %s is not a string: %v", e.Key, e.Value)
}

type KeyNotFoundError struct {
	Key string
}

func (e *KeyNotFoundError) Error() string {
	return fmt.Sprintf("key %s not found in config", e.Key)
}

// Get retrieves a value from the configuration based on the provided key.
// If the value is a string, it is returned as is. If the value is not a string,
// it is converted to a string using fmt.Sprintf and returned along with the error ValueNotStringError.
// If the key is not found in the configuration, an error of type KeyNotFoundError is returned.
func (f *File) Get(key string) (string, error) {
	value, ok := f.Config[key]
	if !ok {
		return "", &KeyNotFoundError{Key: key}
	}

	strValue := fmt.Sprintf("%v", value)

	strValueAsserted, ok := value.(string)
	if !ok {
		return strValue, &ValueNotStringError{Key: key, Value: value}
	}

	return strValueAsserted, nil
}

// Rigel
type Rigel struct {
	Client        *rigel.Rigel
	SchemaName    string
	SchemaVersion int
	ConfigName    string
}

func (r *Rigel) LoadConfig(config any) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.Client.LoadConfig(ctx, r.SchemaName, r.SchemaVersion, r.ConfigName, config)
}

func NewRigelClient(etcdEndpoints string) (*rigel.Rigel, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdEndpoints},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create etcd client: %v", err)
		return nil, err
	}

	etcdStorage := &etcd.EtcdStorage{Client: cli}
	rigelClient := rigel.New(etcdStorage)

	return rigelClient, nil
}

func (f *File) Watch(ctx context.Context, key string, events chan<- Event) error {
	// TODO: Implement the method
	return nil
}

func (r *Rigel) Watch(ctx context.Context, key string, events chan<- Event) error {
	// TODO: Implement the method
	return nil
}
