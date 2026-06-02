package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

type AuthType string

const (
	AuthTypePassword AuthType = "password"
	AuthTypeKey      AuthType = "private_key"
	AuthTypeAgent    AuthType = "ssh_agent"
)

type ConnectionProfile struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	Username          string   `json:"username"`
	AuthType          AuthType `json:"authType"`
	KeyPath           string   `json:"keyPath,omitempty"`
	Description       string   `json:"description,omitempty"`
	PasswordProtected string   `json:"passwordProtected,omitempty"`
	PasswordSaved     bool     `json:"passwordSaved,omitempty"`
}

func (p ConnectionProfile) Validate() error {
	var validationErrors []error

	if strings.TrimSpace(p.ID) == "" {
		validationErrors = append(validationErrors, errors.New("profile id is required"))
	}
	if strings.TrimSpace(p.Name) == "" {
		validationErrors = append(validationErrors, errors.New("profile name is required"))
	}
	if strings.TrimSpace(p.Host) == "" {
		validationErrors = append(validationErrors, errors.New("host is required"))
	} else if strings.ContainsAny(p.Host, "\r\n\t ") {
		validationErrors = append(validationErrors, fmt.Errorf("host contains invalid whitespace: %q", p.Host))
	} else if ip := net.ParseIP(p.Host); ip == nil && !isLikelyDNSName(p.Host) {
		validationErrors = append(validationErrors, fmt.Errorf("host is not a valid IP address or DNS name: %q", p.Host))
	}

	if p.Port < 1 || p.Port > 65535 {
		validationErrors = append(validationErrors, fmt.Errorf("port must be between 1 and 65535: %d", p.Port))
	}
	if strings.TrimSpace(p.Username) == "" {
		validationErrors = append(validationErrors, errors.New("username is required"))
	}

	switch p.AuthType {
	case AuthTypePassword, AuthTypeAgent:
	case AuthTypeKey:
		if strings.TrimSpace(p.KeyPath) == "" {
			validationErrors = append(validationErrors, errors.New("key path is required for private key authentication"))
		}
	default:
		validationErrors = append(validationErrors, fmt.Errorf("unsupported auth type: %q", p.AuthType))
	}

	return errors.Join(validationErrors...)
}

func ValidateProfiles(profiles []ConnectionProfile) error {
	seen := make(map[string]struct{}, len(profiles))
	var validationErrors []error

	for index, profile := range profiles {
		if err := profile.Validate(); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("profile %d: %w", index, err))
		}
		if _, exists := seen[profile.ID]; exists {
			validationErrors = append(validationErrors, fmt.Errorf("profile %d: duplicate profile id %q", index, profile.ID))
		}
		seen[profile.ID] = struct{}{}
	}

	return errors.Join(validationErrors...)
}

func isLikelyDNSName(host string) bool {
	if len(host) > 253 {
		return false
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if r >= 'a' && r <= 'z' {
				continue
			}
			if r >= 'A' && r <= 'Z' {
				continue
			}
			if r >= '0' && r <= '9' {
				continue
			}
			if r == '-' {
				continue
			}
			return false
		}
	}

	return true
}
