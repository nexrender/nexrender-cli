package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallManagedSkill(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "nexrender")
	agent := Agent{Name: "test", Path: destination}
	if err := Install(agent, "1.2.3", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(destination, managedMarker))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(marker), "version=1.2.3") {
		t.Fatalf("unexpected marker: %q", marker)
	}
}

func TestInstallRefusesUnmanagedSkill(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "nexrender")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("manual"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Install(Agent{Name: "test", Path: destination}, "dev", false)
	if err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatalf("expected unmanaged refusal, got %v", err)
	}
}
