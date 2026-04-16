package config_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/remiges-tech/alya/config"
	rigelclient "github.com/remiges-tech/rigel"
	"github.com/remiges-tech/rigel/etcd"
	rigeltypes "github.com/remiges-tech/rigel/types"
)

var (
	_ config.Config   = (*config.File)(nil)
	_ config.Loader   = (*config.File)(nil)
	_ config.Provider = (*config.File)(nil)

	_ config.Config   = (*config.Env)(nil)
	_ config.Loader   = (*config.Env)(nil)
	_ config.Provider = (*config.Env)(nil)

	_ config.Config   = (*config.Rigel)(nil)
	_ config.Loader   = (*config.Rigel)(nil)
	_ config.Provider = (*config.Rigel)(nil)
)

type testAppConfig struct {
	Database struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"database"`
	Server struct {
		Port int `json:"port"`
	} `json:"server"`
	Features struct {
		Enabled bool `json:"enabled"`
	} `json:"features"`
}

type testRigelStorage struct {
	mu       sync.Mutex
	values   map[string]string
	watchers map[string][]chan<- rigeltypes.Event
}

func newTestRigelStorage(values map[string]string) *testRigelStorage {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return &testRigelStorage{
		values:   copied,
		watchers: make(map[string][]chan<- rigeltypes.Event),
	}
}

func (s *testRigelStorage) Get(ctx context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[key], nil
}

func (s *testRigelStorage) Put(ctx context.Context, key string, value string) error {
	s.mu.Lock()
	s.values[key] = value
	watchers := make([]chan<- rigeltypes.Event, 0)
	for prefix, registered := range s.watchers {
		if strings.HasPrefix(key, prefix) {
			watchers = append(watchers, registered...)
		}
	}
	s.mu.Unlock()

	event := rigeltypes.Event{Key: key, Value: value}
	for _, ch := range watchers {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

func (s *testRigelStorage) Watch(_ context.Context, key string, events chan<- rigeltypes.Event) error {
	s.mu.Lock()
	s.watchers[key] = append(s.watchers[key], events)
	s.mu.Unlock()
	return nil
}

func TestNewRigelClient(t *testing.T) {
	etcdEndpoints := "localhost:2379"
	rigelClient, err := config.NewRigelClient(etcdEndpoints)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if rigelClient == nil {
		t.Fatalf("Expected rigelClient to be not nil")
	}

	etcdStorage, ok := rigelClient.Storage.(*etcd.EtcdStorage)
	if !ok {
		t.Fatalf("Expected Storage to be of type *etcd.EtcdStorage")
	}

	if len(etcdStorage.Client.Endpoints()) == 0 || etcdStorage.Client.Endpoints()[0] != etcdEndpoints {
		t.Fatalf("Expected etcdStorage.Client.Endpoints()[0] to be %v, got %v", etcdEndpoints, etcdStorage.Client.Endpoints()[0])
	}
}

func TestGet(t *testing.T) {
	testCases := []struct {
		name          string
		config        map[string]interface{}
		key           string
		expectedValue string
		expectError   bool
	}{
		{
			name:          "Key exists",
			config:        map[string]interface{}{"key1": "value1", "key2": "value2"},
			key:           "key1",
			expectedValue: "value1",
			expectError:   false,
		},
		{
			name:          "Nested key exists",
			config:        map[string]interface{}{"database": map[string]interface{}{"host": "localhost"}},
			key:           "database.host",
			expectedValue: "localhost",
			expectError:   false,
		},
		{
			name:          "Literal dotted key exists",
			config:        map[string]interface{}{"database.host": "literalhost"},
			key:           "database.host",
			expectedValue: "literalhost",
			expectError:   false,
		},
		{
			name:          "Literal dotted key wins over nested key",
			config:        map[string]interface{}{"database.host": "literalhost", "database": map[string]interface{}{"host": "nestedhost"}},
			key:           "database.host",
			expectedValue: "literalhost",
			expectError:   false,
		},
		{
			name:        "Key does not exist",
			config:      map[string]interface{}{"key1": "value1", "key2": "value2"},
			key:         "key3",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file := &config.File{Config: tc.config}

			value, err := file.Get(tc.key)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error but got one: %v", err)
				}
				if value != tc.expectedValue {
					t.Errorf("Expected value %v but got %v", tc.expectedValue, value)
				}
			}
		})
	}
}

func TestLoadWithFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := []byte(`{
  "database": {
    "host": "localhost",
    "port": 5432
  },
  "server": {
    "port": 8084
  },
  "features": {
    "enabled": true
  }
}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	loader, err := config.NewFile(path)
	if err != nil {
		t.Fatalf("NewFile() error = %v", err)
	}

	var cfg testAppConfig
	if err := config.LoadWith(loader, &cfg); err != nil {
		t.Fatalf("LoadWith() error = %v", err)
	}

	if cfg.Database.Host != "localhost" {
		t.Fatalf("Database.Host = %q, want %q", cfg.Database.Host, "localhost")
	}
	if cfg.Database.Port != 5432 {
		t.Fatalf("Database.Port = %d, want %d", cfg.Database.Port, 5432)
	}
	if cfg.Server.Port != 8084 {
		t.Fatalf("Server.Port = %d, want %d", cfg.Server.Port, 8084)
	}
	if !cfg.Features.Enabled {
		t.Fatalf("Features.Enabled = false, want true")
	}

	host, err := loader.Get("database.host")
	if err != nil {
		t.Fatalf("Get(database.host) error = %v", err)
	}
	if host != "localhost" {
		t.Fatalf("Get(database.host) = %q, want %q", host, "localhost")
	}

	port, err := loader.GetInt("database.port")
	if err != nil {
		t.Fatalf("GetInt(database.port) error = %v", err)
	}
	if port != 5432 {
		t.Fatalf("GetInt(database.port) = %d, want %d", port, 5432)
	}

	enabled, err := loader.GetBool("features.enabled")
	if err != nil {
		t.Fatalf("GetBool(features.enabled) error = %v", err)
	}
	if !enabled {
		t.Fatalf("GetBool(features.enabled) = false, want true")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("APP_DATABASE_HOST", "db.example.internal")
	t.Setenv("APP_DATABASE_PORT", "5432")
	t.Setenv("APP_SERVER_PORT", "8085")
	t.Setenv("APP_FEATURES_ENABLED", "true")

	var cfg testAppConfig
	if err := config.LoadConfigFromEnv("APP", &cfg); err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}

	if cfg.Database.Host != "db.example.internal" {
		t.Fatalf("Database.Host = %q, want %q", cfg.Database.Host, "db.example.internal")
	}
	if cfg.Database.Port != 5432 {
		t.Fatalf("Database.Port = %d, want %d", cfg.Database.Port, 5432)
	}
	if cfg.Server.Port != 8085 {
		t.Fatalf("Server.Port = %d, want %d", cfg.Server.Port, 8085)
	}
	if !cfg.Features.Enabled {
		t.Fatalf("Features.Enabled = false, want true")
	}
}

func TestFileWatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig := func(host string) {
		t.Helper()
		content := []byte(`{"database":{"host":"` + host + `","port":5432}}`)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write config file: %v", err)
		}
	}

	writeConfig("localhost")

	loader, err := config.NewFile(path)
	if err != nil {
		t.Fatalf("NewFile() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan config.Event, 1)
	if err := loader.Watch(ctx, "database.host", events); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	writeConfig("db.internal")

	select {
	case event := <-events:
		if event.Key != "database.host" {
			t.Fatalf("Event.Key = %q, want %q", event.Key, "database.host")
		}
		if event.Value != "db.internal" {
			t.Fatalf("Event.Value = %q, want %q", event.Value, "db.internal")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for file watch event")
	}
}

func TestRigelWatch(t *testing.T) {
	schemaFields, err := json.Marshal([]rigeltypes.Field{{Name: "feature_flag", Type: "string"}})
	if err != nil {
		t.Fatalf("marshal schema fields: %v", err)
	}

	storage := newTestRigelStorage(map[string]string{
		rigelclient.GetSchemaFieldsPath("app", "module", 1):                      string(schemaFields),
		rigelclient.GetConfKeyPath("app", "module", 1, "config", "feature_flag"): "off",
	})
	client := rigelclient.New(storage, "app", "module", 1, "config")
	provider := config.NewRigel(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan config.Event, 1)
	if err := provider.Watch(ctx, "feature_flag", events); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	fullKey := rigelclient.GetConfKeyPath("app", "module", 1, "config", "feature_flag")
	if err := storage.Put(ctx, fullKey, "on"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	select {
	case event := <-events:
		if event.Key != "feature_flag" {
			t.Fatalf("Event.Key = %q, want %q", event.Key, "feature_flag")
		}
		if event.Value != "on" {
			t.Fatalf("Event.Value = %q, want %q", event.Value, "on")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for rigel watch event")
	}

	cachedValue, found := client.Cache.Get(fullKey)
	if !found {
		t.Fatal("expected rigel cache to contain updated value")
	}
	if cachedValue != "on" {
		t.Fatalf("cached value = %q, want %q", cachedValue, "on")
	}
}
