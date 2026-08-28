package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultProfile = "default"
	DefaultServer  = "https://api.nexrender.com/api/v2"
)

type Profile struct {
	Server string `json:"server"`
}

type Config struct {
	ActiveProfile string             `json:"activeProfile"`
	Profiles      map[string]Profile `json:"profiles"`
}

type Store struct {
	Path string
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "nexrender", "config.json"), nil
}

func NewDefaultStore() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return &Store{Path: path}, nil
}

func Defaults() Config {
	return Config{
		ActiveProfile: DefaultProfile,
		Profiles: map[string]Profile{
			DefaultProfile: {Server: DefaultServer},
		},
	}
}

func (s *Store) Load() (Config, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = DefaultProfile
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if _, ok := cfg.Profiles[DefaultProfile]; !ok {
		cfg.Profiles[DefaultProfile] = Profile{Server: DefaultServer}
	}
	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func ValidateProfileName(name string) error {
	if name == "" {
		return errors.New("profile name cannot be empty")
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("profile name %q contains unsupported characters", name)
	}
	return nil
}

func NormalizeServer(server string) string {
	return strings.TrimRight(strings.TrimSpace(server), "/")
}
