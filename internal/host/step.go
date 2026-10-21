package host

import (
	"log/slog"
	"os/exec"
	"strings"
)

// Outcome is what one host step found or did (ADR-017 §2).
type Outcome string

const (
	OutcomeOK         Outcome = "ok"         // already in the declared state; nothing done
	OutcomeChanged    Outcome = "changed"    // the system was changed to match
	OutcomeFailed     Outcome = "failed"     // could not converge or read; Fix says why
	OutcomeManual     Outcome = "manual"     // the OS reserves this for the user; Fix names the action
	OutcomeUnverified Outcome = "unverified" // reading the state needs elevation
)

// Succeeded is the legacy SetupStep.Success: the step reached the declared state.
func (o Outcome) Succeeded() bool {
	return o == OutcomeOK || o == OutcomeChanged
}

// Runner issues host commands. It is injected so every OS's step list runs
// under test on any runner (lesson 040); execRunner is the real one.
type Runner interface {
	Run(name string, args ...string) error
	Output(name string, args ...string) ([]byte, error)
	LookPath(name string) (string, error)
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) error {
	// #nosec G204 -- every caller passes a fixed system binary and generated arguments.
	return exec.Command(name, args...).Run()
}

func (execRunner) Output(name string, args ...string) ([]byte, error) {
	// #nosec G204 -- every caller passes a fixed system binary and generated arguments.
	return exec.Command(name, args...).Output()
}

func (execRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func newStep(name string, outcome Outcome, message, fix string) SetupStep {
	return SetupStep{Name: name, Outcome: outcome, Success: outcome.Succeeded(), Message: message, Fix: fix}
}

// setupFor builds and runs the setup steps of goos. darwin has none until #459.
func setupFor(goos string, cfg Config, r Runner) SetupResult {
	switch goos {
	case "windows":
		return setupWindows(cfg, r)
	case "darwin":
		return SetupResult{RDPPort: defaultRDPPort}
	default:
		return setupLinux(cfg, r)
	}
}

// checkFor runs the read-only host check of goos.
func checkFor(goos string, cfg Config, r Runner) CheckResult {
	result := CheckResult{TailscaleIP: tailscaleIP(r)}
	result.TailscaleUp = result.TailscaleIP != ""
	switch goos {
	case "windows":
		checkWindows(cfg, r, &result)
	case "darwin":
		result.RDPPort = defaultRDPPort
	default:
		checkLinux(cfg, r, &result)
	}
	return result
}

func tailscaleIP(r Runner) string {
	out, err := r.Output("tailscale", "ip", "-4")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func tailscaleIPStep(r Runner) SetupStep {
	if ip := tailscaleIP(r); ip != "" {
		return newStep("Tailscale IP", OutcomeOK, "Tailscale IP: "+ip, "")
	}
	return newStep("Tailscale IP", OutcomeFailed, "Tailscale not connected or CLI not available", "start Tailscale and log in")
}

// logSetupSteps emits one structured log line per step. The logger may be nil.
func logSetupSteps(logger *slog.Logger, steps []SetupStep) {
	if logger == nil {
		return
	}
	for _, s := range steps {
		if s.Success {
			logger.Info(s.Name, "outcome", s.Outcome, "message", s.Message)
		} else {
			logger.Warn(s.Name, "outcome", s.Outcome, "message", s.Message, "fix", s.Fix)
		}
	}
}
