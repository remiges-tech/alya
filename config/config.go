package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/remiges-tech/rigel"
	"github.com/remiges-tech/rigel/etcd"
	rigeltypes "github.com/remiges-tech/rigel/types"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Config is an interface that represents a source from which application configuration can be loaded.
//
// Note: this interface is kept for backward compatibility. New code can use Loader for startup
// configuration and Provider for runtime reads.
type Config interface {
	LoadConfig(c any) error
	Check() error
	Get(key string) (string, error)

	// Watch watches for changes to a key in the storage and sends the events to the provided channel.
	// The events includes the key and the updated value.
	// events is the channel to send events when the key's value changes
	Watch(ctx context.Context, key string, events chan<- Event) error
}

// Loader loads startup configuration into a target struct.
type Loader interface {
	Load(target any) error
	Check() error
}

// Provider returns configuration values at the point of use.
// This fits backends that can update values while the process is running.
type Provider interface {
	Get(key string) (string, error)
	GetInt(key string) (int, error)
	GetBool(key string) (bool, error)
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

// LoadWith first ensures that the loader is valid and accessible. Then it loads the config into target.
func LoadWith(loader Loader, target any) error {
	if err := loader.Check(); err != nil {
		return err
	}
	return loader.Load(target)
}

// File is a JSON file-backed config source.
type File struct {
	ConfigFilePath string
	Config         map[string]interface{}
	mu             sync.RWMutex
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

// NewFile creates a file-backed config source.
func NewFile(configFilePath string) (*File, error) {
	return newFile(configFilePath)
}

func (f *File) Load(target any) error {
	return f.LoadConfig(target)
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
//
// Keys can be dot-separated for nested JSON objects, for example database.host.
// If the value is not a string, it is converted to a string using fmt.Sprintf and returned along with
// the error ValueNotStringError.
func (f *File) Get(key string) (string, error) {
	value, err := f.valueForKey(key)
	if err != nil {
		return "", err
	}

	strValue := fmt.Sprintf("%v", value)

	strValueAsserted, ok := value.(string)
	if !ok {
		return strValue, &ValueNotStringError{Key: key, Value: value}
	}

	return strValueAsserted, nil
}

func (f *File) GetInt(key string) (int, error) {
	value, err := f.valueForKey(key)
	if err != nil {
		return 0, err
	}
	return toInt(value, key)
}

func (f *File) GetBool(key string) (bool, error) {
	value, err := f.valueForKey(key)
	if err != nil {
		return false, err
	}
	return toBool(value, key)
}

func (f *File) valueForKey(key string) (any, error) {
	if err := f.ensureConfigLoaded(); err != nil {
		return nil, err
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Preserve direct key lookup before treating dots as nested path separators.
	if value, ok := f.Config[key]; ok {
		return value, nil
	}

	parts := strings.Split(key, ".")
	var current any = f.Config
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, &KeyNotFoundError{Key: key}
		}
		value, ok := m[part]
		if !ok {
			return nil, &KeyNotFoundError{Key: key}
		}
		current = value
	}

	return current, nil
}

func (f *File) ensureConfigLoaded() error {
	f.mu.RLock()
	loaded := f.Config != nil
	f.mu.RUnlock()
	if loaded {
		return nil
	}
	return f.loadConfigMap()
}

func (f *File) loadConfigMap() error {
	file, err := os.Open(f.ConfigFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	configMap := make(map[string]interface{})
	if err := decoder.Decode(&configMap); err != nil {
		return err
	}

	f.mu.Lock()
	f.Config = configMap
	f.mu.Unlock()
	return nil
}

func (f *File) stringValueForKey(key string) (string, bool, error) {
	value, err := f.valueForKey(key)
	if err != nil {
		if _, ok := err.(*KeyNotFoundError); ok {
			return "", false, nil
		}
		return "", false, err
	}
	return fmt.Sprintf("%v", value), true, nil
}

func (f *File) reloadStringValueForKey(key string) (string, bool, error) {
	if err := f.loadConfigMap(); err != nil {
		return "", false, err
	}
	return f.stringValueForKey(key)
}

// Rigel adapts a rigel.Rigel client to Alya's config interfaces.
type Rigel struct {
	Client        *rigel.Rigel
	SchemaName    string
	SchemaVersion int
	ConfigName    string
}

// NewRigel wraps an existing Rigel client so it can be used through Alya's config interfaces.
func NewRigel(client *rigel.Rigel) *Rigel {
	return &Rigel{Client: client}
}

func (r *Rigel) Check() error {
	if r == nil || r.Client == nil {
		return fmt.Errorf("rigel client cannot be nil")
	}
	return nil
}

func (r *Rigel) Load(target any) error {
	return r.LoadConfig(target)
}

func (r *Rigel) LoadConfig(config any) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.Client.LoadConfig(ctx, config)
}

func (r *Rigel) Get(key string) (string, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.Client.Get(ctx, key)
}

func (r *Rigel) GetInt(key string) (int, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.Client.GetInt(ctx, key)
}

func (r *Rigel) GetBool(key string) (bool, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.Client.GetBool(ctx, key)
}

// NewRigelClient creates a Rigel client backed by etcd.
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
	rigelClient := rigel.NewWithStorage(etcdStorage)

	return rigelClient, nil
}

// Watch watches a single logical key in the JSON file.
//
// The file watcher monitors the parent directory so file replacement patterns also work.
// An event is sent only when the resolved string value for key changes.
func (f *File) Watch(ctx context.Context, key string, events chan<- Event) error {
	if err := f.Check(); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if events == nil {
		return fmt.Errorf("events channel cannot be nil")
	}

	prevValue, prevFound, err := f.stringValueForKey(key)
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	configPath := filepath.Clean(f.ConfigFilePath)
	if err := watcher.Add(filepath.Dir(configPath)); err != nil {
		watcher.Close()
		return err
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != configPath {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}

				newValue, newFound, err := f.reloadStringValueForKey(key)
				if err != nil {
					continue
				}
				if newFound == prevFound && newValue == prevValue {
					continue
				}

				prevValue = newValue
				prevFound = newFound

				select {
				case events <- Event{Key: key, Value: newValue}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

// Watch watches a single Rigel config key and forwards updates through Alya events.
func (r *Rigel) Watch(ctx context.Context, key string, events chan<- Event) error {
	if err := r.Check(); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if events == nil {
		return fmt.Errorf("events channel cannot be nil")
	}
	if r.Client.Storage == nil {
		return fmt.Errorf("rigel storage cannot be nil")
	}
	if strings.TrimSpace(r.Client.App) == "" || strings.TrimSpace(r.Client.Module) == "" || r.Client.Version == 0 || strings.TrimSpace(r.Client.Config) == "" {
		return fmt.Errorf("rigel client must include app, module, version, and config for watch")
	}

	fullKey := rigel.GetConfKeyPath(r.Client.App, r.Client.Module, r.Client.Version, r.Client.Config, key)
	rigelEvents := make(chan rigeltypes.Event, 1)
	if err := r.Client.Storage.Watch(ctx, fullKey, rigelEvents); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-rigelEvents:
				if !ok {
					return
				}
				if r.Client.Cache != nil {
					r.Client.Cache.Set(fullKey, event.Value)
				}
				select {
				case events <- Event{Key: key, Value: event.Value}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

func toInt(value any, key string) (int, error) {
	s := fmt.Sprintf("%v", value)
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("value for key %s is not an int: %v", key, value)
	}
	return i, nil
}

func toBool(value any, key string) (bool, error) {
	s := fmt.Sprintf("%v", value)
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("value for key %s is not a bool: %v", key, value)
	}
	return b, nil
}
