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
