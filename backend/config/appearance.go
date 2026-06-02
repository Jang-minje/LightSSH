package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type AppearanceSettings struct {
	Theme              string `json:"theme"`
	AppFontFamily      string `json:"appFontFamily"`
	AppFontSize        int    `json:"appFontSize"`
	TerminalFontFamily string `json:"terminalFontFamily"`
	TerminalFontSize   int    `json:"terminalFontSize"`
}

type AppearanceStore struct {
	path string
}

func DefaultAppearanceSettings() AppearanceSettings {
	return AppearanceSettings{
		Theme:              "light",
		AppFontFamily:      `"Malgun Gothic", "Segoe UI", sans-serif`,
		AppFontSize:        14,
		TerminalFontFamily: `"Cascadia Mono", Consolas, monospace`,
		TerminalFontSize:   14,
	}
}

func NewDefaultAppearanceStore() (*AppearanceStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config directory: %w", err)
	}
	return &AppearanceStore{path: filepath.Join(dir, "LightSSH", "settings.json")}, nil
}

func (s *AppearanceStore) Load() (AppearanceSettings, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultAppearanceSettings(), nil
	}
	if err != nil {
		return AppearanceSettings{}, fmt.Errorf("read settings file: %w", err)
	}
	var settings AppearanceSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return AppearanceSettings{}, fmt.Errorf("parse settings file: %w", err)
	}
	return normalizeAppearanceSettings(settings), nil
}

func (s *AppearanceStore) Save(settings AppearanceSettings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	data, err := json.MarshalIndent(normalizeAppearanceSettings(settings), "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings file: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}
	return nil
}

func normalizeAppearanceSettings(settings AppearanceSettings) AppearanceSettings {
	defaults := DefaultAppearanceSettings()
	if settings.Theme != "dark" {
		settings.Theme = "light"
	}
	if settings.AppFontFamily == "" {
		settings.AppFontFamily = defaults.AppFontFamily
	}
	if settings.TerminalFontFamily == "" {
		settings.TerminalFontFamily = defaults.TerminalFontFamily
	}
	settings.AppFontSize = clampInt(settings.AppFontSize, 12, 20, defaults.AppFontSize)
	settings.TerminalFontSize = clampInt(settings.TerminalFontSize, 10, 28, defaults.TerminalFontSize)
	return settings
}

func clampInt(value int, min int, max int, fallback int) int {
	if value == 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
