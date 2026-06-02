package tunnel

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"lightssh/backend/config"
	"lightssh/backend/sshclient"
)

func TestManagerListenerLifecycle(t *testing.T) {
	manager := NewManager(&mockSSHDialer{target: &mockTargetDialer{}})
	request := StartRequest{
		SSH: sshclient.ConnectRequest{
			Profile: config.ConnectionProfile{
				ID:       "dev",
				Name:     "Development",
				Host:     "ssh.internal",
				Port:     22,
				Username: "ops",
				AuthType: config.AuthTypePassword,
			},
			Password: "secret",
		},
		Config: LocalConfig{
			ID:               "local-db",
			Name:             "Local DB",
			LocalBindAddress: "127.0.0.1",
			LocalPort:        0,
			TargetHost:       "db.internal",
			TargetPort:       5432,
		},
	}

	info, err := manager.Start(context.Background(), request)
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if info.Status != "running" {
		t.Fatalf("status = %q, want running", info.Status)
	}
	if info.LocalPort == 0 {
		t.Fatal("expected assigned local port")
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(info.LocalAddress, strconv.Itoa(info.LocalPort)), time.Second)
	if err != nil {
		t.Fatalf("dial local tunnel: %v", err)
	}
	_, _ = conn.Write([]byte("ping"))
	_ = conn.Close()

	if err := manager.Stop(info.TunnelID); err != nil {
		t.Fatalf("stop tunnel: %v", err)
	}
	if _, err := net.DialTimeout("tcp", net.JoinHostPort(info.LocalAddress, strconv.Itoa(info.LocalPort)), 100*time.Millisecond); err == nil {
		t.Fatal("expected listener to be closed")
	}
}

func TestManagerDetectsPortConflict(t *testing.T) {
	manager := NewManager(&mockSSHDialer{target: &mockTargetDialer{}})
	base := StartRequest{
		SSH: sshclient.ConnectRequest{
			Profile: config.ConnectionProfile{
				ID:       "dev",
				Name:     "Development",
				Host:     "ssh.internal",
				Port:     22,
				Username: "ops",
				AuthType: config.AuthTypePassword,
			},
			Password: "secret",
		},
		Config: LocalConfig{
			ID:               "one",
			Name:             "One",
			LocalBindAddress: "127.0.0.1",
			LocalPort:        0,
			TargetHost:       "db.internal",
			TargetPort:       5432,
		},
	}

	info, err := manager.Start(context.Background(), base)
	if err != nil {
		t.Fatalf("start first tunnel: %v", err)
	}
	defer manager.Stop(info.TunnelID)

	base.Config.ID = "two"
	base.Config.Name = "Two"
	base.Config.LocalPort = info.LocalPort
	if _, err := manager.Start(context.Background(), base); err == nil {
		t.Fatal("expected port conflict")
	}
}

func TestManagerReverseListenerLifecycle(t *testing.T) {
	target := &mockTargetDialer{}
	manager := NewManager(&mockSSHDialer{target: target})
	request := StartReverseRequest{
		SSH: sshclient.ConnectRequest{
			Profile: config.ConnectionProfile{
				ID:       "dev",
				Name:     "Development",
				Host:     "ssh.internal",
				Port:     22,
				Username: "ops",
				AuthType: config.AuthTypePassword,
			},
			Password: "secret",
		},
		Config: ReverseConfig{
			ID:                "reverse-db",
			Name:              "Reverse DB",
			RemoteBindAddress: "127.0.0.1",
			RemotePort:        15432,
			LocalTargetHost:   "127.0.0.1",
			LocalTargetPort:   5432,
		},
	}

	info, err := manager.StartReverse(context.Background(), request)
	if err != nil {
		t.Fatalf("start reverse tunnel: %v", err)
	}
	if info.Direction != "reverse" {
		t.Fatalf("direction = %q, want reverse", info.Direction)
	}
	if !target.listenCalled {
		t.Fatal("expected remote listen through target dialer")
	}

	if err := manager.Stop(info.TunnelID); err != nil {
		t.Fatalf("stop reverse tunnel: %v", err)
	}
	if !target.closed {
		t.Fatal("expected target dialer to close")
	}
}

func TestManagerReverseListenErrorIsClear(t *testing.T) {
	manager := NewManager(&mockSSHDialer{target: &mockTargetDialer{listenErr: errors.New("ssh: tcpip-forward request denied administratively prohibited")}})
	request := StartReverseRequest{
		SSH: sshclient.ConnectRequest{
			Profile: config.ConnectionProfile{
				ID:       "dev",
				Name:     "Development",
				Host:     "ssh.internal",
				Port:     22,
				Username: "ops",
				AuthType: config.AuthTypePassword,
			},
			Password: "secret",
		},
		Config: ReverseConfig{
			ID:                "reverse-db",
			Name:              "Reverse DB",
			RemoteBindAddress: "127.0.0.1",
			RemotePort:        15432,
			LocalTargetHost:   "127.0.0.1",
			LocalTargetPort:   5432,
		},
	}

	_, err := manager.StartReverse(context.Background(), request)
	if err == nil {
		t.Fatal("expected reverse listen error")
	}
	if !strings.Contains(err.Error(), "remote forwarding is not allowed") {
		t.Fatalf("error = %q, want remote forwarding not allowed", err.Error())
	}
}

type mockSSHDialer struct {
	target TargetDialer
}

func (m *mockSSHDialer) Dial(context.Context, sshclient.ConnectRequest) (TargetDialer, error) {
	return m.target, nil
}

type mockTargetDialer struct {
	listenCalled bool
	listenErr    error
	closed       bool
}

func (m *mockTargetDialer) Dial(string, string) (net.Conn, error) {
	left, right := net.Pipe()
	go func() {
		_, _ = io.Copy(io.Discard, right)
		_ = right.Close()
	}()
	return left, nil
}

func (m *mockTargetDialer) Listen(string, string) (net.Listener, error) {
	m.listenCalled = true
	if m.listenErr != nil {
		return nil, m.listenErr
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

func (m *mockTargetDialer) Close() error {
	m.closed = true
	return nil
}
