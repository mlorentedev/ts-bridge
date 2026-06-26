package cmd_test

import (
	"strings"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
)

func TestBuildDescriptorLine_SaaSControl(t *testing.T) {
	got := cmdpkg.BuildDescriptorLineForTest("100.64.0.1", 3389, "")
	if !strings.Contains(got, "tsb://100.64.0.1:3389") {
		t.Errorf("expected tsb:// descriptor, got %q", got)
	}
	if !strings.Contains(got, "cp=saas") {
		t.Errorf("expected cp=saas for empty control URL, got %q", got)
	}
}

func TestBuildDescriptorLine_HeadscaleControl(t *testing.T) {
	got := cmdpkg.BuildDescriptorLineForTest("100.64.0.1", 3389, "https://vpn.example.com")
	if !strings.Contains(got, "cp=https") {
		t.Errorf("expected cp= with Headscale URL in %q", got)
	}
}

func TestBuildDescriptorLine_EmptyIP_ReturnsEmpty(t *testing.T) {
	got := cmdpkg.BuildDescriptorLineForTest("", 3389, "")
	if got != "" {
		t.Errorf("expected empty string for empty IP, got %q", got)
	}
}

func TestBuildDescriptorLine_ZeroPort_ReturnsEmpty(t *testing.T) {
	got := cmdpkg.BuildDescriptorLineForTest("100.64.0.1", 0, "")
	if got != "" {
		t.Errorf("expected empty string for zero port, got %q", got)
	}
}
