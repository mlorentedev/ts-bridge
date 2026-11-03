package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ts-bridge/internal/health"
	"ts-bridge/internal/host"
	"ts-bridge/internal/serve"
)

// serveProbeClient reads host serve's health endpoint on loopback.
var serveProbeClient = &http.Client{Timeout: 2 * time.Second}

// serviceKey is how CheckResult.Published names a service.
func serviceKey(kind string, port int) string {
	return fmt.Sprintf("%s:%d", kind, port)
}

// probeServePublished asks host serve, on host.mesh.health_addr, which of the
// declared services it publishes (HOST-008 AC6). It returns nil when the file
// declares no health_addr. Serve not answering, or a responder that does not
// name the file's node, reads as nothing published.
func probeServePublished(file host.File, declared []host.DeclaredService) map[string]bool {
	mesh := file.Host.Mesh
	if mesh == nil || mesh.HealthAddr == nil {
		return nil
	}
	published := make(map[string]bool, len(declared))
	for _, s := range declared {
		published[serviceKey(s.Kind, s.Port)] = false
	}
	resp, err := serveProbeClient.Get("http://" + *mesh.HealthAddr + "/health/ready")
	if err != nil {
		return published
	}
	defer resp.Body.Close()
	var body struct {
		Node     string                `json:"node"`
		Services []health.ServiceState `json:"services"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil || body.Node != serve.NodeHostname(*mesh) {
		return published
	}
	for _, s := range body.Services {
		if _, declared := published[serviceKey(s.Kind, s.Port)]; declared && s.Published {
			published[serviceKey(s.Kind, s.Port)] = true
		}
	}
	return published
}
