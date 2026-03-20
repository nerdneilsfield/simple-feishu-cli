package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AppID     string
	AppSecret string
}

type LoadOptions struct {
	AppID      string
	AppSecret  string
	ConfigPath string
	HomeDir    string
}

type fileConfig struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

func Load(opts LoadOptions) (Config, error) {
	cfg := Config{}

	if value := strings.TrimSpace(os.Getenv("FEISHU_APP_ID")); value != "" {
		cfg.AppID = value
	}
	if value := strings.TrimSpace(os.Getenv("FEISHU_APP_SECRET")); value != "" {
		cfg.AppSecret = value
	}
	if value := strings.TrimSpace(opts.AppID); value != "" {
		cfg.AppID = value
	}
	if value := strings.TrimSpace(opts.AppSecret); value != "" {
		cfg.AppSecret = value
	}

	explicitPath := strings.TrimSpace(opts.ConfigPath) != ""
	if explicitPath || !hasCredentials(cfg) {
		path, err := resolveConfigPath(opts)
		if err != nil {
			if !explicitPath && hasCredentials(cfg) {
				return cfg, nil
			}
			return Config{}, err
		}

		fileCfg, err := loadFileConfig(path, explicitPath)
		if err != nil {
			if !explicitPath && hasCredentials(cfg) {
				return cfg, nil
			}
			return Config{}, err
		}

		if cfg.AppID == "" {
			cfg.AppID = strings.TrimSpace(fileCfg.AppID)
		}
		if cfg.AppSecret == "" {
			cfg.AppSecret = strings.TrimSpace(fileCfg.AppSecret)
		}
	}

	var missing []string
	if cfg.AppID == "" {
		missing = append(missing, "app_id")
	}
	if cfg.AppSecret == "" {
		missing = append(missing, "app_secret")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required credentials: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func DefaultConfigPath(home string) (string, error) {
	home = strings.TrimSpace(home)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home dir: %w", err)
		}
	}

	return filepath.Join(home, ".config", "feishu", "config.yaml"), nil
}

func resolveConfigPath(opts LoadOptions) (string, error) {
	if value := strings.TrimSpace(opts.ConfigPath); value != "" {
		return value, nil
	}

	return DefaultConfigPath(opts.HomeDir)
}

func hasCredentials(cfg Config) bool {
	return strings.TrimSpace(cfg.AppID) != "" && strings.TrimSpace(cfg.AppSecret) != ""
}

func loadFileConfig(path string, explicit bool) (fileConfig, error) {
	if path == "" {
		return fileConfig{}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if explicit {
				return fileConfig{}, fmt.Errorf("config path %q does not exist", path)
			}
			return fileConfig{}, nil
		}
		return fileConfig{}, fmt.Errorf("stat config file %q: %w", path, err)
	}

	if info.IsDir() {
		return fileConfig{}, fmt.Errorf("config path %q is a directory", path)
	}

	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fileConfig{}, fmt.Errorf("config file %q has insecure permissions %03o; use 0600", path, info.Mode().Perm())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	var cfg fileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fileConfig{}, fmt.Errorf("parse config file %q: %w", path, err)
	}

	return cfg, nil
}
