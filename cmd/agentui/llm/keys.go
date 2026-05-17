package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KeySource indicates where the API key came from (used by the GUI to label
// the current state).
type KeySource string

const (
	KeyFromEnv  KeySource = "env"
	KeyFromFile KeySource = "file"
	KeyAbsent   KeySource = ""
)

// KeyStatus is the GUI-facing description of the current API-key state.
type KeyStatus struct {
	Configured bool      `json:"configured"`
	Source     KeySource `json:"source"`
	Tail       string    `json:"tail,omitempty"`   // last 4 chars of key, just for visual confirmation
	BaseURL    string    `json:"baseURL,omitempty"` // empty = official Anthropic endpoint
}

const (
	envVar        = "ANTHROPIC_API_KEY"
	envBaseURL    = "ANTHROPIC_BASE_URL"
	configDir     = ".mobilevc"
	configFile    = "agentui-config.json"
	keyField      = "anthropicApiKey"
	baseURLField  = "anthropicBaseURL"
)

// LoadKey returns the active API key (env > file). Empty + KeyAbsent if not set.
func LoadKey() (string, KeySource) {
	if k := strings.TrimSpace(os.Getenv(envVar)); k != "" {
		return k, KeyFromEnv
	}
	k, _ := readField(keyField)
	if strings.TrimSpace(k) == "" {
		return "", KeyAbsent
	}
	return k, KeyFromFile
}

// LoadBaseURL returns the active base URL (env > file). Empty = use default.
func LoadBaseURL() string {
	if u := strings.TrimSpace(os.Getenv(envBaseURL)); u != "" {
		return u
	}
	u, _ := readField(baseURLField)
	return strings.TrimSpace(u)
}

// Status assembles a KeyStatus describing the current setup.
func Status() KeyStatus {
	k, src := LoadKey()
	bu := LoadBaseURL()
	if k == "" {
		return KeyStatus{Configured: false, Source: KeyAbsent, BaseURL: bu}
	}
	st := KeyStatus{Configured: true, Source: src, BaseURL: bu}
	if len(k) > 4 {
		st.Tail = k[len(k)-4:]
	}
	return st
}

// SaveConfig writes both key and base URL atomically to the config file
// (0600). Empty string for either field clears that entry.
func SaveConfig(key, baseURL string) error {
	key = strings.TrimSpace(key)
	baseURL = strings.TrimSpace(baseURL)
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	cfg := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(existing, &cfg)
	}
	if key == "" {
		delete(cfg, keyField)
	} else {
		cfg[keyField] = key
	}
	if baseURL == "" {
		delete(cfg, baseURLField)
	} else {
		cfg[baseURLField] = baseURL
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return writeFileMode(path, data, 0600)
}

func readField(name string) (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", err
	}
	if v, ok := cfg[name].(string); ok {
		return v, nil
	}
	return "", errors.New("field not set")
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, configDir, configFile), nil
}

// writeFileMode is os.WriteFile but enforces the given mode even when the
// file already existed (os.WriteFile only sets mode on first create).
func writeFileMode(path string, data []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
