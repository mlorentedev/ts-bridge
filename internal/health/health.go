package health

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
	"ts-bridge/internal/telemetry"
)

// statusField is the JSON key returned by the health endpoints.
const statusField = "status"

// TunnelStatus is a thread-safe ready flag with an optional reason string.
// Use Store to set state; Load to read it. Zero value is ready with no reason.
type TunnelStatus struct {
	ready  atomic.Bool
	reason atomic.Value // stores string
}

// Store sets the ready flag and reason atomically.
func (s *TunnelStatus) Store(ready bool, reason string) {
	s.reason.Store(reason)
	s.ready.Store(ready)
}

// Load returns the current ready state and reason.
func (s *TunnelStatus) Load() (ready bool, reason string) {
	r, _ := s.reason.Load().(string)
	return s.ready.Load(), r
}

// MarkReady sets the status to ready with no reason (startup complete).
func (s *TunnelStatus) MarkReady() { s.Store(true, "") }

// StartServer initializes and runs the health and metrics HTTP server.
func StartServer(addr string, status *TunnelStatus, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{statusField: "ok"})
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ready, reason := status.Load()
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			body := map[string]string{statusField: "not_ready"}
			if reason != "" {
				body["reason"] = reason
			}
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{statusField: "ok"})
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		snapshot := telemetry.GetMetrics()
		_ = json.NewEncoder(w).Encode(snapshot)
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("health server starting", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server error", "error", err)
		}
	}()

	return server
}
