// Package httpapi exposes the registry facade over HTTP.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flashbacks/registry/internal/secrets"
	"github.com/flashbacks/registry/internal/store"
)

const watchTimeout = 30 * time.Second

// Server serves the registry REST facade.
type Server struct {
	store     store.Store
	codec     *secrets.Codec
	authToken string
}

// New creates a facade server.
func New(st store.Store, codec *secrets.Codec, authToken string) *Server {
	return &Server{store: st, codec: codec, authToken: authToken}
}

type valueRequest struct {
	Value string `json:"value"`
}

type instanceRequest struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	BasePath   string `json:"basePath,omitempty"`
	HealthPath string `json:"healthPath,omitempty"`
	Version    string `json:"version,omitempty"`
}

// Handler returns the HTTP handler for the facade.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /registry/v1/health", s.health)

	mux.HandleFunc("PUT /registry/v1/config/{service}/{key}", s.putConfig)
	mux.HandleFunc("GET /registry/v1/config/{service}", s.listConfig)
	mux.HandleFunc("GET /registry/v1/config/{service}/watch", s.watchConfig)
	mux.HandleFunc("GET /registry/v1/config/{service}/{key}", s.getConfig)
	mux.HandleFunc("DELETE /registry/v1/config/{service}/{key}", s.deleteConfig)

	mux.HandleFunc("PUT /registry/v1/secrets/{service}/{key}", s.putSecret)
	mux.HandleFunc("GET /registry/v1/secrets/{service}/{key}", s.getSecret)
	mux.HandleFunc("DELETE /registry/v1/secrets/{service}/{key}", s.deleteSecret)

	mux.HandleFunc("PUT /registry/v1/services/{service}/{instance}", s.registerService)
	mux.HandleFunc("POST /registry/v1/services/{service}/{instance}/heartbeat", s.heartbeatService)
	mux.HandleFunc("DELETE /registry/v1/services/{service}/{instance}", s.deregisterService)
	mux.HandleFunc("GET /registry/v1/services/{service}", s.listServices)
	mux.HandleFunc("GET /registry/v1/services/{service}/watch", s.watchServices)

	return s.withAuth(mux)
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authToken != "" && r.URL.Path != "/registry/v1/health" {
			if r.Header.Get("Authorization") != "Bearer "+s.authToken {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if _, _, err := s.store.Get(ctx, store.RootPrefix()+".health"); err != nil {
		writeError(w, http.StatusServiceUnavailable, "backend unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	service, key := r.PathValue("service"), r.PathValue("key")
	if err := validateSegments(service, key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req valueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Put(r.Context(), store.ConfigKey(service, key), req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	service, key := r.PathValue("service"), r.PathValue("key")
	if err := validateSegments(service, key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	value, found, err := s.store.Get(r.Context(), store.ConfigKey(service, key))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "config key not found")
		return
	}
	writeJSON(w, http.StatusOK, valueRequest{Value: value})
}

func (s *Server) listConfig(w http.ResponseWriter, r *http.Request) {
	service := r.PathValue("service")
	if err := validateSegments(service); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	prefix := store.ConfigPrefix(service)
	values, err := s.store.List(r.Context(), prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[strings.TrimPrefix(key, prefix)] = value
	}
	writeJSON(w, http.StatusOK, map[string]any{"values": out})
}

func (s *Server) deleteConfig(w http.ResponseWriter, r *http.Request) {
	service, key := r.PathValue("service"), r.PathValue("key")
	if err := validateSegments(service, key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Delete(r.Context(), store.ConfigKey(service, key)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) putSecret(w http.ResponseWriter, r *http.Request) {
	service, key := r.PathValue("service"), r.PathValue("key")
	if err := validateSegments(service, key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req valueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	encrypted, err := s.codec.Encrypt(req.Value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.Put(r.Context(), store.SecretKey(service, key), encrypted); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) getSecret(w http.ResponseWriter, r *http.Request) {
	service, key := r.PathValue("service"), r.PathValue("key")
	if err := validateSegments(service, key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	encrypted, found, err := s.store.Get(r.Context(), store.SecretKey(service, key))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}
	plaintext, err := s.codec.Decrypt(encrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, valueRequest{Value: plaintext})
}

func (s *Server) deleteSecret(w http.ResponseWriter, r *http.Request) {
	service, key := r.PathValue("service"), r.PathValue("key")
	if err := validateSegments(service, key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Delete(r.Context(), store.SecretKey(service, key)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) registerService(w http.ResponseWriter, r *http.Request) {
	service, instance := r.PathValue("service"), r.PathValue("instance")
	if err := validateSegments(service, instance); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req instanceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Host == "" {
		writeError(w, http.StatusBadRequest, "host is required")
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		writeError(w, http.StatusBadRequest, "port must be between 1 and 65535")
		return
	}
	ttl := int64(15)
	if raw := r.URL.Query().Get("ttl"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "ttl must be a positive integer")
			return
		}
		ttl = parsed
	}
	payload, err := json.Marshal(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	leaseID, err := s.store.PutWithLease(r.Context(), store.ServiceKey(service, instance), string(payload), ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"leaseId": leaseID, "ttl": ttl})
}

func (s *Server) heartbeatService(w http.ResponseWriter, r *http.Request) {
	service, instance := r.PathValue("service"), r.PathValue("instance")
	if err := validateSegments(service, instance); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		LeaseID int64 `json:"leaseId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.LeaseID == 0 {
		writeError(w, http.StatusBadRequest, "leaseId is required")
		return
	}
	ttl, err := s.store.KeepAliveOnce(r.Context(), req.LeaseID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"leaseId": req.LeaseID, "ttl": ttl})
}

func (s *Server) deregisterService(w http.ResponseWriter, r *http.Request) {
	service, instance := r.PathValue("service"), r.PathValue("instance")
	if err := validateSegments(service, instance); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Delete(r.Context(), store.ServiceKey(service, instance)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listServices(w http.ResponseWriter, r *http.Request) {
	service := r.PathValue("service")
	if err := validateSegments(service); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	prefix := store.ServicesPrefix(service)
	values, err := s.store.List(r.Context(), prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	instances := make([]map[string]any, 0, len(values))
	for key, value := range values {
		var inst instanceRequest
		if err := json.Unmarshal([]byte(value), &inst); err != nil {
			continue
		}
		item := map[string]any{
			"id":   strings.TrimPrefix(key, prefix),
			"host": inst.Host,
			"port": inst.Port,
		}
		if inst.BasePath != "" {
			item["basePath"] = inst.BasePath
		}
		if inst.HealthPath != "" {
			item["healthPath"] = inst.HealthPath
		}
		if inst.Version != "" {
			item["version"] = inst.Version
		}
		instances = append(instances, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"instances": instances})
}

func (s *Server) watchConfig(w http.ResponseWriter, r *http.Request) {
	service := r.PathValue("service")
	if err := validateSegments(service); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.stream(w, r, store.ConfigPrefix(service))
}

func (s *Server) watchServices(w http.ResponseWriter, r *http.Request) {
	service := r.PathValue("service")
	if err := validateSegments(service); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.stream(w, r, store.ServicesPrefix(service))
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request, prefix string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), watchTimeout)
	defer cancel()

	events, err := s.store.Watch(ctx, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func validateSegments(segments ...string) error {
	for _, segment := range segments {
		if segment == "" {
			return fmt.Errorf("path segment is required")
		}
		if strings.Contains(segment, "/") {
			return fmt.Errorf("path segment %q must not contain '/'", segment)
		}
	}
	return nil
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
