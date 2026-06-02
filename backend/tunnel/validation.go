package tunnel

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

func (c LocalConfig) Validate() error {
	var validationErrors []error

	if strings.TrimSpace(c.ID) == "" {
		validationErrors = append(validationErrors, errors.New("tunnel id is required"))
	}
	if strings.TrimSpace(c.Name) == "" {
		validationErrors = append(validationErrors, errors.New("tunnel name is required"))
	}
	if strings.TrimSpace(c.LocalBindAddress) == "" {
		validationErrors = append(validationErrors, errors.New("local bind address is required"))
	} else if net.ParseIP(c.LocalBindAddress) == nil && c.LocalBindAddress != "localhost" {
		validationErrors = append(validationErrors, fmt.Errorf("invalid local bind address: %q", c.LocalBindAddress))
	}
	if c.LocalPort < 0 || c.LocalPort > 65535 {
		validationErrors = append(validationErrors, fmt.Errorf("local port must be between 0 and 65535: %d", c.LocalPort))
	}
	if strings.TrimSpace(c.TargetHost) == "" {
		validationErrors = append(validationErrors, errors.New("target host is required"))
	} else if strings.ContainsAny(c.TargetHost, "\r\n\t ") {
		validationErrors = append(validationErrors, fmt.Errorf("target host contains invalid whitespace: %q", c.TargetHost))
	}
	if c.TargetPort < 1 || c.TargetPort > 65535 {
		validationErrors = append(validationErrors, fmt.Errorf("target port must be between 1 and 65535: %d", c.TargetPort))
	}

	return errors.Join(validationErrors...)
}

func (c ReverseConfig) Validate() error {
	var validationErrors []error

	if strings.TrimSpace(c.ID) == "" {
		validationErrors = append(validationErrors, errors.New("tunnel id is required"))
	}
	if strings.TrimSpace(c.Name) == "" {
		validationErrors = append(validationErrors, errors.New("tunnel name is required"))
	}
	if strings.TrimSpace(c.RemoteBindAddress) == "" {
		validationErrors = append(validationErrors, errors.New("remote bind address is required"))
	} else if net.ParseIP(c.RemoteBindAddress) == nil && c.RemoteBindAddress != "localhost" {
		validationErrors = append(validationErrors, fmt.Errorf("invalid remote bind address: %q", c.RemoteBindAddress))
	}
	if c.RemotePort < 1 || c.RemotePort > 65535 {
		validationErrors = append(validationErrors, fmt.Errorf("remote port must be between 1 and 65535: %d", c.RemotePort))
	}
	if strings.TrimSpace(c.LocalTargetHost) == "" {
		validationErrors = append(validationErrors, errors.New("local target host is required"))
	} else if strings.ContainsAny(c.LocalTargetHost, "\r\n\t ") {
		validationErrors = append(validationErrors, fmt.Errorf("local target host contains invalid whitespace: %q", c.LocalTargetHost))
	}
	if c.LocalTargetPort < 1 || c.LocalTargetPort > 65535 {
		validationErrors = append(validationErrors, fmt.Errorf("local target port must be between 1 and 65535: %d", c.LocalTargetPort))
	}

	return errors.Join(validationErrors...)
}
