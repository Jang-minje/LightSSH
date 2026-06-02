package tunnel

import (
	"context"
	"net"

	"lightssh/backend/sshclient"

	"golang.org/x/crypto/ssh"
)

type TargetDialer interface {
	Dial(network string, address string) (net.Conn, error)
	Listen(network string, address string) (net.Listener, error)
	Close() error
}

type SSHDialer interface {
	Dial(ctx context.Context, request sshclient.ConnectRequest) (TargetDialer, error)
}

type SSHConnector interface {
	DialClient(ctx context.Context, request sshclient.ConnectRequest) (*ssh.Client, error)
}

type SSHConnectorDialer struct {
	connector SSHConnector
}

func NewSSHConnectorDialer(connector SSHConnector) *SSHConnectorDialer {
	return &SSHConnectorDialer{connector: connector}
}

func (d *SSHConnectorDialer) Dial(ctx context.Context, request sshclient.ConnectRequest) (TargetDialer, error) {
	return d.connector.DialClient(ctx, request)
}

type LocalConfig struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	LocalBindAddress string `json:"localBindAddress"`
	LocalPort        int    `json:"localPort"`
	TargetHost       string `json:"targetHost"`
	TargetPort       int    `json:"targetPort"`
}

type StartRequest struct {
	SSH    sshclient.ConnectRequest `json:"ssh"`
	Config LocalConfig              `json:"config"`
}

type ReverseConfig struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	RemoteBindAddress string `json:"remoteBindAddress"`
	RemotePort        int    `json:"remotePort"`
	LocalTargetHost   string `json:"localTargetHost"`
	LocalTargetPort   int    `json:"localTargetPort"`
}

type StartReverseRequest struct {
	SSH    sshclient.ConnectRequest `json:"ssh"`
	Config ReverseConfig            `json:"config"`
}

type Info struct {
	TunnelID        string `json:"tunnelId"`
	Direction       string `json:"direction"`
	Name            string `json:"name"`
	LocalAddress    string `json:"localAddress"`
	LocalPort       int    `json:"localPort"`
	RemoteAddress   string `json:"remoteAddress"`
	RemotePort      int    `json:"remotePort"`
	TargetHost      string `json:"targetHost"`
	TargetPort      int    `json:"targetPort"`
	Status          string `json:"status"`
	ConnectionCount int    `json:"connectionCount"`
	Message         string `json:"message,omitempty"`
}

type Event struct {
	TunnelID        string `json:"tunnelId"`
	Status          string `json:"status"`
	ConnectionCount int    `json:"connectionCount"`
	Message         string `json:"message,omitempty"`
}
