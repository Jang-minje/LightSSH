package sshclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Manager struct {
	connector Connector

	mu       sync.RWMutex
	sessions map[string]TerminalSession
}

func NewManager(connector Connector) *Manager {
	return &Manager{
		connector: connector,
		sessions:  make(map[string]TerminalSession),
	}
}

func (m *Manager) Start(ctx context.Context, request ConnectRequest) (TerminalSession, error) {
	if request.Columns <= 0 {
		request.Columns = DefaultColumns
	}
	if request.Rows <= 0 {
		request.Rows = DefaultRows
	}

	session, err := m.connector.Connect(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("connect ssh session: %w", err)
	}

	m.mu.Lock()
	m.sessions[session.ID()] = session
	m.mu.Unlock()

	return session, nil
}

func (m *Manager) Write(sessionID string, data string) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	return session.Write(data)
}

func (m *Manager) KeepAlive(sessionID string) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	return session.KeepAlive()
}

func (m *Manager) Resize(sessionID string, columns int, rows int) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	if columns <= 0 || rows <= 0 {
		return errors.New("terminal size must be positive")
	}
	return session.Resize(columns, rows)
}

func (m *Manager) Close(sessionID string) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}

	m.remove(sessionID)
	return session.Close()
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	sessions := make([]TerminalSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.sessions = make(map[string]TerminalSession)
	m.mu.Unlock()

	for _, session := range sessions {
		_ = session.Close()
	}
}

func (m *Manager) Forget(sessionID string) {
	m.remove(sessionID)
}

func (m *Manager) get(sessionID string) (TerminalSession, error) {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("ssh session not found: %s", sessionID)
	}
	return session, nil
}

func (m *Manager) remove(sessionID string) {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}
