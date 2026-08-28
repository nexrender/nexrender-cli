package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	store := &Store{Path: path}
	cfg := Defaults()
	cfg.ActiveProfile = "staging"
	cfg.Profiles["staging"] = Profile{Server: "https://example.test/api/v2"}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ActiveProfile != "staging" || loaded.Profiles["staging"].Server != "https://example.test/api/v2" {
		t.Fatalf("unexpected config: %#v", loaded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
}

func TestValidateProfileName(t *testing.T) {
	for _, valid := range []string{"default", "customer-a", "customer_2"} {
		if err := ValidateProfileName(valid); err != nil {
			t.Fatalf("expected %q to be valid: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "customer a", "../escape"} {
		if err := ValidateProfileName(invalid); err == nil {
			t.Fatalf("expected %q to be invalid", invalid)
		}
	}
}
