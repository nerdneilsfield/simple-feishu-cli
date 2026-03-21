package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadPrefersFlagsOverEnvAndFile(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "app_id: file-id\napp_secret: file-secret\n", 0o600)

	t.Setenv("FEISHU_APP_ID", "env-id")
	t.Setenv("FEISHU_APP_SECRET", "env-secret")

	cfg, err := Load(LoadOptions{
		AppID:      "flag-id",
		AppSecret:  "flag-secret",
		ConfigPath: path,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppID != "flag-id" || cfg.AppSecret != "flag-secret" {
		t.Fatalf("Load() = %#v, want flags to win", cfg)
	}
}

func TestLoadPrefersEnvOverFile(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "app_id: file-id\napp_secret: file-secret\n", 0o600)

	t.Setenv("FEISHU_APP_ID", "env-id")
	t.Setenv("FEISHU_APP_SECRET", "env-secret")

	cfg, err := Load(LoadOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppID != "env-id" || cfg.AppSecret != "env-secret" {
		t.Fatalf("Load() = %#v, want env to win over file", cfg)
	}
}

func TestLoadReadsDefaultConfigPath(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "feishu")
	path := writeConfigFile(t, configDir, "app_id: file-id\napp_secret: file-secret\n", 0o600)

	cfg, err := Load(LoadOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppID != "file-id" || cfg.AppSecret != "file-secret" {
		t.Fatalf("Load() = %#v, want values from %s", cfg, path)
	}
}

func TestLoadReturnsClearErrorWhenCredentialsMissing(t *testing.T) {
	_, err := Load(LoadOptions{})
	if err == nil {
		t.Fatal("Load() error = nil, want missing credential error")
	}

	if !strings.Contains(err.Error(), "app_id") || !strings.Contains(err.Error(), "app_secret") {
		t.Fatalf("Load() error = %q, want both app_id and app_secret mentioned", err)
	}
}

func TestLoadIgnoresBrokenDefaultConfigWhenHigherPriorityCredentialsExist(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "feishu")
	writeConfigFile(t, configDir, "app_id: [broken\n", 0o600)

	cfg, err := Load(LoadOptions{
		AppID:     "flag-id",
		AppSecret: "flag-secret",
		HomeDir:   home,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppID != "flag-id" || cfg.AppSecret != "flag-secret" {
		t.Fatalf("Load() = %#v, want flag credentials", cfg)
	}
}

func TestLoadReturnsExplicitMissingConfigPathError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := Load(LoadOptions{ConfigPath: path})
	if err == nil {
		t.Fatal("Load() error = nil, want explicit config path error")
	}

	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "config path") {
		t.Fatalf("Load() error = %q, want explicit config path failure", err)
	}
}

func TestLoadRejectsInsecureConfigPermissionsOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission check is not enforced on Windows")
	}

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "app_id: file-id\napp_secret: file-secret\n", 0o644)

	_, err := Load(LoadOptions{ConfigPath: path})
	if err == nil {
		t.Fatal("Load() error = nil, want insecure permission error")
	}

	if !strings.Contains(err.Error(), "0600") {
		t.Fatalf("Load() error = %q, want permission guidance", err)
	}
}

func writeConfigFile(t *testing.T, dir, content string, mode os.FileMode) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("Chmod(%q) error = %v", path, err)
	}

	return path
}
