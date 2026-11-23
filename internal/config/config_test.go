package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL || cfg.Username != "ARTEX" {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"base_url":"http://file:8787/","username":"ARTEX","password":"fromfile"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARTEX_URL", "http://env:9000")
	t.Setenv("ARTEX_PASSWORD", "fromenv")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != "http://env:9000" || cfg.Password != "fromenv" {
		t.Fatalf("env did not win: %+v", cfg)
	}
}

func TestTrailingSlashTrimmed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"base_url":"http://x:8787///"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://x:8787" {
		t.Fatalf("base = %q", cfg.BaseURL)
	}
}

func TestInvalidJSONIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected parse error")
	}
}
