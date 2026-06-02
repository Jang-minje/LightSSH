package sshclient

import (
	"context"

	"lightssh/backend/config"
)

const (
	DefaultColumns = 80
	DefaultRows    = 24
)

type ConnectRequest struct {
	Profile        config.ConnectionProfile `json:"profile"`
	Password       string                   `json:"password,omitempty"`
	Passphrase     string                   `json:"passphrase,omitempty"`
	Columns        int                      `json:"columns"`
	Rows           int                      `json:"rows"`
	TimeoutSeconds int                      `json:"timeoutSeconds"`
}

type HostKeyPrompt struct {
	RequestID      string `json:"requestId"`
	Host           string `json:"host"`
	Address        string `json:"address"`
	KeyType        string `json:"keyType"`
	Fingerprint    string `json:"fingerprint"`
	KnownHosts     string `json:"knownHosts"`
	KnownHostsLine string `json:"-"`
	Changed        bool   `json:"changed"`
}

type HostKeyDecision struct {
	RequestID string `json:"requestId"`
	Accept    bool   `json:"accept"`
}

type SessionInfo struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

type OutputEvent struct {
	SessionID string `json:"sessionId"`
	Stream    string `json:"stream"`
	Data      string `json:"data"`
}

type CloseEvent struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

type Connector interface {
	Connect(ctx context.Context, request ConnectRequest) (TerminalSession, error)
}

type TerminalSession interface {
	ID() string
	Output() <-chan OutputEvent
	Done() <-chan error
	Write(data string) error
	KeepAlive() error
	Resize(columns int, rows int) error
	Close() error
}
