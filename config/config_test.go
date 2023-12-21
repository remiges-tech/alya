package config_test

import (
	"testing"

	"github.com/remiges-tech/alya/config"
	"github.com/remiges-tech/rigel/etcd"
)

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
	// Define test cases
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
			name:        "Key does not exist",
			config:      map[string]interface{}{"key1": "value1", "key2": "value2"},
			key:         "key3",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new File with the test case config
			file := &config.File{Config: tc.config}

			// Call Get and check the result
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
