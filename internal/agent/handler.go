package agent

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// apiVersion is the only API version this agent serves. /v1/meta publishes it
// as a range so a client released apart from the agent can tell skew.
const apiVersion = 1

// apiError is the one error shape: Code is stable for programs, Message is for
// people, Detail names the input that caused it.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (a *Agent) handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/meta", getOnly(a.meta))
	mux.Handle("/v1/status", getOnly(a.status))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, apiError{Code: "not_found", Message: "no such endpoint", Detail: r.URL.Path})
	})
	return mux
}

func (a *Agent) meta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_version": a.version,
		"api_min":       apiVersion,
		"api_max":       apiVersion,
	})
}

func (a *Agent) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"pid":     os.Getpid(),
		"started": a.started.Format(time.RFC3339),
	})
}

// getOnly answers any other method with the error envelope, not ServeMux's
// plain-text 405.
func getOnly(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, apiError{Code: "method_not_allowed", Message: "this endpoint only answers GET", Detail: r.Method})
			return
		}
		h(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
