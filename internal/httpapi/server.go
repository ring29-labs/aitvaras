package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ring29-labs/aitvaras/internal/core"
	"github.com/ring29-labs/aitvaras/internal/model"
)

type Server struct {
	service *core.Service
	token   string
}

func New(service *core.Service, token string) http.Handler {
	s := &Server{service: service, token: token}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.Handle("POST /v1/nodes", s.authorize(http.HandlerFunc(s.registerNode)))
	mux.Handle("GET /v1/nodes", s.authorize(http.HandlerFunc(s.listNodes)))
	mux.Handle("POST /v1/intents", s.authorize(http.HandlerFunc(s.createIntent)))
	mux.Handle("GET /v1/intents/{id}", s.authorize(http.HandlerFunc(s.getIntent)))
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "ring29-control"})
}

func (s *Server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if provided == r.Header.Get("Authorization") || subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) registerNode(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterNodeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	node, err := s.service.RegisterNode(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) listNodes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"nodes": s.service.ListNodes()})
}

func (s *Server) createIntent(w http.ResponseWriter, r *http.Request) {
	var req model.CreateIntentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	intent, err := s.service.CreateIntent(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, intent)
}

func (s *Server) getIntent(w http.ResponseWriter, r *http.Request) {
	intent, err := s.service.GetIntent(r.PathValue("id"))
	if errors.Is(err, core.ErrNotFound) {
		writeError(w, http.StatusNotFound, "intent not found")
		return
	}
	writeJSON(w, http.StatusOK, intent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return errors.New("invalid JSON body: " + err.Error())
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid JSON body: multiple values")
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
