package budgetapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/ring29-labs/aitvaras/internal/budget"
)

func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func failure(w http.ResponseWriter, err error) {
	status := http.StatusServiceUnavailable
	switch {
	case errors.Is(err, budget.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, budget.ErrDenied):
		status = http.StatusForbidden
	case errors.Is(err, budget.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, budget.ErrNotFound):
		status = http.StatusNotFound
	}
	reply(w, status, map[string]string{"error": err.Error()})
}

func authenticate(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "Bearer " + token
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(want)) != 1 {
			reply(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Node(ledger *budget.Ledger, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/budget", func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := ledger.Snapshot()
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, http.StatusOK, snapshot)
	})
	mux.HandleFunc("GET /v1/budget/reservations/{id}", func(w http.ResponseWriter, r *http.Request) {
		receipt, err := ledger.Get(r.PathValue("id"))
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, http.StatusOK, receipt)
	})
	mux.HandleFunc("POST /v1/budget/reservations", func(w http.ResponseWriter, r *http.Request) {
		var req budget.Request
		if err := budget.Decode(http.MaxBytesReader(w, r.Body, 8192), &req); err != nil {
			failure(w, budget.ErrInvalid)
			return
		}
		receipt, err := ledger.Reserve(req)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, http.StatusOK, receipt)
	})
	return authenticate(token, mux)
}

// Control routes only to a fixed operator-configured local Node. Registration
// metadata and client inputs never select the upstream destination.
func Control(nodeURL, clientToken, nodeToken string) (http.Handler, error) {
	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, errors.New("invalid budget node URL")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "http" || ip == nil || !ip.IsLoopback() || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("budget node URL must be an HTTP numeric loopback origin")
	}
	if strings.TrimSpace(clientToken) == "" || strings.TrimSpace(nodeToken) == "" || clientToken == nodeToken {
		return nil, errors.New("distinct client and node tokens required")
	}
	u.Path = ""
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			r.Out.Header.Set("Authorization", "Bearer "+nodeToken)
		},
		Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, ResponseHeaderTimeout: 5 * time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConnsPerHost: 8},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			reply(w, http.StatusBadGateway, map[string]string{"error": "node outcome unknown; query or retry the same request ID and fields"})
		},
	}
	return authenticate(clientToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})), nil
}
