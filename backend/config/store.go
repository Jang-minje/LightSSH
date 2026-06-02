package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Store struct {
	path string
}

type fileFormat struct {
	Profiles []ConnectionProfile `json:"profiles"`
}

func NewStore(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("config path is required")
	}

	return &Store{path: path}, nil
}

func NewDefaultStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config directory: %w", err)
	}

	return NewStore(filepath.Join(dir, "LightSSH", "profiles.json"))
}

func (s *Store) LoadProfiles() ([]ConnectionProfile, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []ConnectionProfile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var file fileFormat
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	if err := ValidateProfiles(file.Profiles); err != nil {
		return nil, fmt.Errorf("validate config file: %w", err)
	}

	return file.Profiles, nil
}

func (s *Store) SaveProfiles(profiles []ConnectionProfile) error {
	if err := ValidateProfiles(profiles); err != nil {
		return fmt.Errorf("validate profiles: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(fileFormat{Profiles: profiles}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config file: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}
