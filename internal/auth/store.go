package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

const serviceName = "nexrender-cli"

type Credential struct {
	Token  string `json:"-"`
	Source string `json:"source"`
}

type Store interface {
	Get(profile string) (Credential, error)
	Set(profile, token string) (string, error)
	Delete(profile string) error
}

type DefaultStore struct {
	FilePath string
}

func NewDefaultStore() (*DefaultStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("find user config directory: %w", err)
	}
	return &DefaultStore{FilePath: filepath.Join(dir, "nexrender", "credentials.json")}, nil
}

func (s *DefaultStore) Get(profile string) (Credential, error) {
	if token := strings.TrimSpace(os.Getenv("NEXRENDER_API_KEY")); token != "" {
		return Credential{Token: token, Source: "environment"}, nil
	}
	if token, err := keyring.Get(serviceName, profile); err == nil && strings.TrimSpace(token) != "" {
		return Credential{Token: token, Source: "keyring"}, nil
	}
	values, err := s.readFile()
	if err != nil {
		return Credential{}, err
	}
	if token := strings.TrimSpace(values[profile]); token != "" {
		return Credential{Token: token, Source: "file"}, nil
	}
	return Credential{}, os.ErrNotExist
}

func (s *DefaultStore) Set(profile, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("token cannot be empty")
	}
	if err := keyring.Set(serviceName, profile, token); err == nil {
		return "keyring", nil
	}
	values, err := s.readFile()
	if err != nil {
		return "", err
	}
	values[profile] = token
	if err := s.writeFile(values); err != nil {
		return "", err
	}
	return "file", nil
}

func (s *DefaultStore) Delete(profile string) error {
	_ = keyring.Delete(serviceName, profile)
	values, err := s.readFile()
	if err != nil {
		return err
	}
	delete(values, profile)
	return s.writeFile(values)
}

func (s *DefaultStore) readFile() (map[string]string, error) {
	data, err := os.ReadFile(s.FilePath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	values := map[string]string{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return values, nil
}

func (s *DefaultStore) writeFile(values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(s.FilePath), 0o700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	data = append(data, '\n')
	tmp := s.FilePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Rename(tmp, s.FilePath); err != nil {
		return fmt.Errorf("replace credentials: %w", err)
	}
	return nil
}
