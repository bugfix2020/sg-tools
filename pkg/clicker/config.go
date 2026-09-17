package clicker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PersistedProfile struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Bindings []KeyBinding `json:"bindings"`
}

type AppConfig struct {
	Version    int                `json:"version"`
	Hotkeys    HotkeyConfig       `json:"hotkeys"`
	Profiles   []PersistedProfile `json:"profiles"`
	FollowSync KeyRuleConfig      `json:"followSync"`
}

func DefaultConfig() AppConfig {
	return AppConfig{
		Version: 1,
		Hotkeys: DefaultHotkeys(),
		Profiles: []PersistedProfile{{
			ID:       "profile-1",
			Name:     "Tab 1",
			Bindings: make([]KeyBinding, MaxBindings),
		}},
	}
}

type ConfigStore struct {
	path string
}

func NewConfigStore(path string) *ConfigStore { return &ConfigStore{path: path} }

func (s *ConfigStore) Load() (AppConfig, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return DefaultConfig(), err
	}
	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return DefaultConfig(), fmt.Errorf("decode config: %w", err)
	}
	if config.Version != 1 {
		return DefaultConfig(), fmt.Errorf("unsupported config version %d", config.Version)
	}
	if err := ValidateHotkeys(config.Hotkeys); err != nil {
		return DefaultConfig(), fmt.Errorf("invalid hotkeys in config: %w", err)
	}
	return config, nil
}

func (s *ConfigStore) Save(config AppConfig) error {
	if config.Version == 0 {
		config.Version = 1
	}
	if err := ValidateHotkeys(config.Hotkeys); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

func (s *ConfigStore) writeRaw(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}
