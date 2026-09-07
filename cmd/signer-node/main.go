package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ring29-labs/aitvaras/internal/model"
	"github.com/ring29-labs/aitvaras/internal/node"
)

func main() {
	config, ok := loadConfig()
	if !ok {
		os.Exit(1)
	}
	registration := model.RegisterNodeRequest{
		ID: config.id, TenantID: config.tenantID, PublicURL: config.publicURL,
		Chains: []string{"eip155:84532", "solana:devnet"}, Capabilities: []string{"registration_only"},
	}
	client := node.Client{CoreURL: config.coreURL, Token: config.token, HTTP: &http.Client{Timeout: 10 * time.Second}}
	registered, err := client.Register(context.Background(), registration)
	if err != nil {
		slog.Error("could not register with Ring29 Control", "error", err)
		os.Exit(1)
	}
	slog.Info("node registered", "node_id", registered.ID, "tenant_id", registered.TenantID)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "ring29-node"})
	})
	mux.HandleFunc("GET /v1/node", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, registered) })
	mux.HandleFunc("POST /v1/sign", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "signing is not implemented in the inception node"})
	})

	server := &http.Server{Addr: config.address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	slog.Info("Ring29 Node listening", "address", config.address, "capability", "registration_only")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("node server stopped", "error", err)
		os.Exit(1)
	}
}

type config struct{ coreURL, token, id, tenantID, publicURL, address string }

func loadConfig() (config, bool) {
	c := config{
		coreURL: os.Getenv("RING29_CORE_URL"), token: os.Getenv("RING29_DEV_TOKEN"),
		id: os.Getenv("RING29_NODE_ID"), tenantID: os.Getenv("RING29_TENANT_ID"),
		publicURL: os.Getenv("RING29_NODE_PUBLIC_URL"), address: envOr("RING29_NODE_ADDR", "127.0.0.1:8081"),
	}
	missing := make([]string, 0)
	for name, value := range map[string]string{
		"RING29_CORE_URL": c.coreURL, "RING29_DEV_TOKEN": c.token, "RING29_NODE_ID": c.id,
		"RING29_TENANT_ID": c.tenantID, "RING29_NODE_PUBLIC_URL": c.publicURL,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		slog.Error("missing required environment variables", "names", strings.Join(missing, ","))
		return config{}, false
	}
	return c, true
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
