package sftpclient

import (
	"context"

	"lightssh/backend/sshclient"

	"golang.org/x/crypto/ssh"
)

type SSHDialer interface {
	DialClient(ctx context.Context, request sshclient.ConnectRequest) (*ssh.Client, error)
}

type ConnectRequest struct {
	SSH sshclient.ConnectRequest `json:"ssh"`
}

type SessionInfo struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

type RemoteEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
}

type LocalEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

type TransferRequest struct {
	SessionID  string `json:"sessionId"`
	LocalPath  string `json:"localPath"`
	RemotePath string `json:"remotePath"`
	Overwrite  bool   `json:"overwrite"`
}

type TerminalTransferRequest struct {
	SSHSessionID string `json:"sshSessionId"`
	LocalPath    string `json:"localPath"`
	RemotePath   string `json:"remotePath"`
	Overwrite    bool   `json:"overwrite"`
}

type TransferInfo struct {
	TransferID string `json:"transferId"`
	Status     string `json:"status"`
}

type ProgressEvent struct {
	TransferID string `json:"transferId"`
	SessionID  string `json:"sessionId"`
	Direction  string `json:"direction"`
	LocalPath  string `json:"localPath"`
	RemotePath string `json:"remotePath"`
	Bytes      int64  `json:"bytes"`
	Total      int64  `json:"total"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}
