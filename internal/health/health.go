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

// ServiceState is one service host serve publishes, as /health/ready reports it.
type ServiceState struct {
	Kind      string `json:"kind"`
	Port      int    `json:"port"`
	Published bool   `json:"published"`
}

// Report returns the node name and its services, added to /health/ready's
// body so host check can tell what serve publishes (HOST-008 AC6).
type Report func() (node string, services []ServiceState)

// StartServer initializes and runs the health and metrics HTTP server.
func StartServer(addr string, status *TunnelStatus, logger *slog.Logger) *http.Server {
	return StartServerWithReport(addr, status, nil, logger)
}

// StartServerWithReport is StartServer with a Report on /health/ready.
func StartServerWithReport(addr string, status *TunnelStatus, report Report, logger *slog.Logger) *http.Server {
	server := &http.Server{
		Addr:              addr,
		Handler:           Handler(status, report),
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

// Handler serves /health/live, /health/ready and /metrics. A nil report keeps
// connect's /health/ready body unchanged.
func Handler(status *TunnelStatus, report Report) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{statusField: "ok"})
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ready, reason := status.Load()
		body := map[string]any{statusField: "ok"}
		code := http.StatusOK
		if !ready {
			code = http.StatusServiceUnavailable
			body[statusField] = "not_ready"
			if reason != "" {
				body["reason"] = reason
			}
		}
		if report != nil {
			body["node"], body["services"] = report()
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(body)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		snapshot := telemetry.GetMetrics()
		_ = json.NewEncoder(w).Encode(snapshot)
	})

	return mux
}
