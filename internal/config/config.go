// Package config loads the artex-mcp connection settings.
//
// Precedence: flags > environment > config file > defaults. The config file is
// deliberately small: it only carries what the bridge needs to authenticate
// against a locally deployed ARTEX instance.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultBaseURL is the address ARTEX listens on when started with start.sh.
const DefaultBaseURL = "http://127.0.0.1:8787"

// Config holds everything needed to reach ARTEX.
type Config struct {
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

// DefaultPath returns the config file location, honouring ARTEX_MCP_CONFIG.
func DefaultPath() string {
	if p := strings.TrimSpace(os.Getenv("ARTEX_MCP_CONFIG")); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "artex-mcp", "config.json")
}

// Load reads the config file (which may be absent) and applies environment
// overrides. An absent file is not an error as long as the environment supplies
// enough to authenticate.
func Load(path string) (Config, error) {
	cfg := Config{BaseURL: DefaultBaseURL, Username: "ARTEX"}

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := json.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
			}
		case errors.Is(err, os.ErrNotExist):
			// Fine: the bridge can be driven purely by environment variables.
		default:
			return cfg, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
		}
	}

	applyEnv(&cfg)

	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Username = strings.TrimSpace(cfg.Username)
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Username == "" {
		cfg.Username = "ARTEX"
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("ARTEX_URL")); v != "" {
		cfg.BaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("ARTEX_USERNAME")); v != "" {
		cfg.Username = v
	}
	if v := os.Getenv("ARTEX_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := strings.TrimSpace(os.Getenv("ARTEX_TOKEN")); v != "" {
		cfg.Token = v
	}
}
