// policy-node runs the keyless budget_policy_v0 Node capability.
package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ring29-labs/aitvaras/internal/budget"
	"github.com/ring29-labs/aitvaras/internal/budgetapi"
)

func run() error {
	policyPath := flag.String("policy", "examples/budget-policy.json", "operator-owned policy file")
	journal := flag.String("journal", "budget.journal", "existing local reservation journal")
	initialize := flag.Bool("init", false, "initialize a new journal and exit; never overwrite")
	address := flag.String("addr", "127.0.0.1:8082", "loopback listen address")
	flag.Parse()
	f, err := os.Open(*policyPath)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(data) > 16384 {
		return errors.New("policy file unreadable or exceeds 16384 bytes")
	}
	var policy budget.Policy
	if err := budget.Decode(bytes.NewReader(data), &policy); err != nil {
		return errors.New("invalid policy JSON")
	}
	ledger, err := budget.Open(*journal, policy, *initialize)
	if err != nil {
		return err
	}
	defer ledger.Close()
	if *initialize {
		slog.Info("budget journal initialized; no funds or keys created")
		return nil
	}
	token := os.Getenv("RING29_NODE_TOKEN")
	if token == "" {
		return errors.New("RING29_NODE_TOKEN is required")
	}
	host, _, err := net.SplitHostPort(*address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("policy-node requires a numeric loopback listen address")
	}
	server := &http.Server{Addr: *address, Handler: budgetapi.Node(ledger, token), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 8 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	slog.Info("Aitvaras Policy Node listening", "address", *address, "role", "budget_policy_v0", "mode", "reservation-only; no signing")
	return server.ListenAndServe()
}

func main() {
	if err := run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("policy-node stopped", "error", err)
		os.Exit(1)
	}
}
