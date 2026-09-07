package node_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/ring29-labs/aitvaras/internal/core"
	"github.com/ring29-labs/aitvaras/internal/httpapi"
	"github.com/ring29-labs/aitvaras/internal/model"
	"github.com/ring29-labs/aitvaras/internal/node"
)

func TestRegister(t *testing.T) {
	server := httptest.NewServer(httpapi.New(core.New(), "token"))
	defer server.Close()
	client := node.Client{CoreURL: server.URL, Token: "token", HTTP: server.Client()}
	registered, err := client.Register(context.Background(), model.RegisterNodeRequest{
		ID: "node-1", TenantID: "tenant-1", PublicURL: "http://127.0.0.1:8081", Chains: []string{"eip155:84532"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered.ID != "node-1" {
		t.Fatalf("got id %q", registered.ID)
	}
}
