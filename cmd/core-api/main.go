package main

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ring29-labs/aitvaras/internal/budgetapi"
	"github.com/ring29-labs/aitvaras/internal/core"
	"github.com/ring29-labs/aitvaras/internal/httpapi"
)

func main() {
	token := os.Getenv("RING29_DEV_TOKEN")
	if token == "" {
		slog.Error("RING29_DEV_TOKEN is required; the static token is for local development only")
		os.Exit(1)
	}
	address := envOr("RING29_CONTROL_ADDR", "127.0.0.1:8080")
	mux := http.NewServeMux()
	mux.Handle("/", httpapi.New(core.New(), token))
	nodeURL, clientToken, nodeToken := os.Getenv("RING29_BUDGET_NODE_URL"), os.Getenv("RING29_CLIENT_TOKEN"), os.Getenv("RING29_NODE_TOKEN")
	if nodeURL != "" || clientToken != "" || nodeToken != "" {
		host, _, err := net.SplitHostPort(address)
		if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || token == clientToken || token == nodeToken {
			slog.Error("budget profile requires loopback and separate admin, client and node tokens")
			os.Exit(1)
		}
		handler, err := budgetapi.Control(nodeURL, clientToken, nodeToken)
		if err != nil {
			slog.Error("invalid budget routing configuration", "error", err)
			os.Exit(1)
		}
		mux.Handle("/v1/budget", handler)
		mux.Handle("/v1/budget/", handler)
	}
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 8 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	slog.Info("Ring29 Control listening", "address", address, "budget_forwarding", nodeURL != "", "warning", "legacy state in memory; budget state at Node; no signing")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("control server stopped", "error", err)
		os.Exit(1)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
