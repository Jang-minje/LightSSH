package sshclient

import (
	"context"
	"errors"
	"testing"

	"lightssh/backend/config"
)

func TestManagerStartStoresSessionAndDelegatesOperations(t *testing.T) {
	mockSession := newMockSession("session-1")
	connector := &mockConnector{session: mockSession}
	manager := NewManager(connector)

	request := ConnectRequest{
		Profile: config.ConnectionProfile{
			ID:       "dev",
			Name:     "Development",
			Host:     "dev.example.internal",
			Port:     22,
			Username: "ops",
			AuthType: config.AuthTypePassword,
		},
		Password: "secret",
	}

	session, err := manager.Start(context.Background(), request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if session.ID() != "session-1" {
		t.Fatalf("session id = %q, want session-1", session.ID())
	}
	if connector.request.Password != "secret" {
		t.Fatal("manager did not pass transient password to connector")
	}
	if err := manager.Write("session-1", "ls\r"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if mockSession.written != "ls\r" {
		t.Fatalf("written = %q, want ls\\r", mockSession.written)
	}
	if err := manager.Resize("session-1", 120, 40); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if mockSession.columns != 120 || mockSession.rows != 40 {
		t.Fatalf("size = %dx%d, want 120x40", mockSession.columns, mockSession.rows)
	}
	if err := manager.Close("session-1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !mockSession.closed {
		t.Fatal("session was not closed")
	}
}

func TestManagerStartReturnsConnectorError(t *testing.T) {
	manager := NewManager(&mockConnector{err: errors.New("authentication failed")})

	_, err := manager.Start(context.Background(), ConnectRequest{})
	if err == nil {
		t.Fatal("expected connector error")
	}
}

func TestManagerResizeRejectsInvalidSize(t *testing.T) {
	mockSession := newMockSession("session-1")
	manager := NewManager(&mockConnector{session: mockSession})

	if _, err := manager.Start(context.Background(), ConnectRequest{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := manager.Resize("session-1", 0, 24); err == nil {
		t.Fatal("expected invalid size error")
	}
}

type mockConnector struct {
	session TerminalSession
	request ConnectRequest
	err     error
}

func (m *mockConnector) Connect(_ context.Context, request ConnectRequest) (TerminalSession, error) {
	m.request = request
	if m.err != nil {
		return nil, m.err
	}
	return m.session, nil
}

type mockSession struct {
	id      string
	output  chan OutputEvent
	done    chan error
	written string
	columns int
	rows    int
	closed  bool
}

func newMockSession(id string) *mockSession {
	return &mockSession{
		id:     id,
		output: make(chan OutputEvent),
		done:   make(chan error),
	}
}

func (m *mockSession) ID() string {
	return m.id
}

func (m *mockSession) Output() <-chan OutputEvent {
	return m.output
}

func (m *mockSession) Done() <-chan error {
	return m.done
}

func (m *mockSession) Write(data string) error {
	m.written += data
	return nil
}

func (m *mockSession) KeepAlive() error {
	return nil
}

func (m *mockSession) Resize(columns int, rows int) error {
	m.columns = columns
	m.rows = rows
	return nil
}

func (m *mockSession) Close() error {
	m.closed = true
	close(m.done)
	return nil
}
