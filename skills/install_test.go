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

func TestInstallRemovesFilesFromAnOlderSkill(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "nexrender")
	agent := Agent{Name: "test", Path: destination}
	if err := Install(agent, "1.0.0", false); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(destination, "references", "removed.md")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(agent, "1.1.0", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale file survived the update: %v", err)
	}
}

func TestInstallKeepsASymlinkedSkillFolder(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "shared")
	link := filepath.Join(root, "nexrender")
	if err := Install(Agent{Name: "test", Path: target}, "1.0.0", false); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := Install(Agent{Name: "test", Path: link}, "1.1.0", false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the skill folder symlink was replaced by a directory")
	}
	marker, err := os.ReadFile(filepath.Join(target, managedMarker))
	if err != nil || !strings.Contains(string(marker), "version=1.1.0") {
		t.Fatalf("symlink target was not updated: %q, %v", marker, err)
	}
}

func TestManagedListsOnlyCLIInstalledSkills(t *testing.T) {
	home := t.TempDir()
	known := KnownAgents(home)
	if err := Install(known[2], "1.0.0", false); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(known[1].Path, 0o755); err != nil {
		t.Fatal(err)
	}
	managed := Managed(home)
	if len(managed) != 1 || managed[0].Name != known[2].Name {
		t.Fatalf("managed = %+v", managed)
	}
}
