package sftpclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	sftp "github.com/pkg/sftp/v2"
	"golang.org/x/crypto/ssh"
)

type Manager struct {
	dialer SSHDialer

	mu        sync.RWMutex
	sessions  map[string]*Session
	transfers map[string]context.CancelFunc
	progress  chan ProgressEvent
}

type Session struct {
	id        string
	sshClient *ssh.Client
	client    *sftp.Client
}

func NewManager(dialer SSHDialer) *Manager {
	return &Manager{
		dialer:    dialer,
		sessions:  make(map[string]*Session),
		transfers: make(map[string]context.CancelFunc),
		progress:  make(chan ProgressEvent, 128),
	}
}

func (m *Manager) Progress() <-chan ProgressEvent {
	return m.progress
}

func (m *Manager) Start(ctx context.Context, request ConnectRequest) (SessionInfo, error) {
	sshClient, err := m.dialer.DialClient(ctx, request.SSH)
	if err != nil {
		return SessionInfo{}, fmt.Errorf("connect ssh for sftp: %w", err)
	}

	client, err := sftp.NewClient(ctx, sshClient)
	if err != nil {
		_ = sshClient.Close()
		return SessionInfo{}, fmt.Errorf("create sftp client: %w", err)
	}

	sessionID, err := newID()
	if err != nil {
		_ = client.Close()
		_ = sshClient.Close()
		return SessionInfo{}, err
	}

	m.mu.Lock()
	m.sessions[sessionID] = &Session{
		id:        sessionID,
		sshClient: sshClient,
		client:    client,
	}
	m.mu.Unlock()

	return SessionInfo{SessionID: sessionID, Status: "connected"}, nil
}

func (m *Manager) Close(sessionID string) error {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	for transferID, cancel := range m.transfers {
		if strings.HasPrefix(transferID, sessionID+":") {
			cancel()
			delete(m.transfers, transferID)
		}
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("sftp session not found: %s", sessionID)
	}

	err := session.client.Close()
	if closeErr := session.sshClient.Close(); err == nil {
		err = closeErr
	}
	return err
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.sessions = make(map[string]*Session)
	for transferID, cancel := range m.transfers {
		cancel()
		delete(m.transfers, transferID)
	}
	m.mu.Unlock()

	for _, session := range sessions {
		_ = session.client.Close()
		_ = session.sshClient.Close()
	}
}

func (m *Manager) ListRemote(sessionID string, remotePath string) ([]RemoteEntry, error) {
	session, err := m.get(sessionID)
	if err != nil {
		return nil, err
	}
	cleanPath, err := cleanRemotePath(remotePath)
	if err != nil {
		return nil, err
	}

	entries, err := session.client.ReadDir(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read remote directory: %w", err)
	}

	result := make([]RemoteEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("read remote entry info: %w", err)
		}
		result = append(result, RemoteEntry{
			Name:    entry.Name(),
			Path:    path.Join(cleanPath, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

func ListLocal(localPath string) ([]LocalEntry, error) {
	cleanPath, err := cleanLocalPath(localPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read local directory: %w", err)
	}

	result := make([]LocalEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("read local entry info: %w", err)
		}
		result = append(result, LocalEntry{
			Name:    entry.Name(),
			Path:    filepath.Join(cleanPath, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

func (m *Manager) Mkdir(sessionID string, remotePath string) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	cleanPath, err := cleanRemotePath(remotePath)
	if err != nil {
		return err
	}
	return session.client.Mkdir(cleanPath, 0o755)
}

func (m *Manager) Rename(sessionID string, oldPath string, newPath string) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	cleanOld, err := cleanRemotePath(oldPath)
	if err != nil {
		return err
	}
	cleanNew, err := cleanRemotePath(newPath)
	if err != nil {
		return err
	}
	return session.client.Rename(cleanOld, cleanNew)
}

func (m *Manager) Delete(sessionID string, remotePath string) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	cleanPath, err := cleanRemotePath(remotePath)
	if err != nil {
		return err
	}
	info, err := session.client.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("stat remote path: %w", err)
	}
	if info.IsDir() {
		return session.client.Remove(cleanPath)
	}
	return session.client.Remove(cleanPath)
}

func (m *Manager) Upload(request TransferRequest) (TransferInfo, error) {
	session, err := m.get(request.SessionID)
	if err != nil {
		return TransferInfo{}, err
	}
	localPath, err := cleanLocalPath(request.LocalPath)
	if err != nil {
		return TransferInfo{}, err
	}
	remotePath, err := cleanRemotePath(request.RemotePath)
	if err != nil {
		return TransferInfo{}, err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return TransferInfo{}, fmt.Errorf("stat local file: %w", err)
	}
	if info.IsDir() {
		return TransferInfo{}, errors.New("upload source must be a file")
	}
	if remoteInfo, err := session.client.Stat(remotePath); err == nil && remoteInfo.IsDir() {
		remotePath = path.Join(remotePath, filepath.Base(localPath))
	}
	if !request.Overwrite {
		if _, err := session.client.Stat(remotePath); err == nil {
			return TransferInfo{}, fmt.Errorf("remote file already exists: %s", remotePath)
		}
	}

	transferID, ctx, cancel, err := m.registerTransfer(request.SessionID)
	if err != nil {
		return TransferInfo{}, err
	}

	go m.upload(ctx, cancel, transferID, request.SessionID, localPath, remotePath, info.Size(), session.client)

	return TransferInfo{TransferID: transferID, Status: "started"}, nil
}

func (m *Manager) Download(request TransferRequest) (TransferInfo, error) {
	session, err := m.get(request.SessionID)
	if err != nil {
		return TransferInfo{}, err
	}
	remotePath, err := cleanRemotePath(request.RemotePath)
	if err != nil {
		return TransferInfo{}, err
	}
	localPath, err := cleanLocalPath(request.LocalPath)
	if err != nil {
		return TransferInfo{}, err
	}

	info, err := session.client.Stat(remotePath)
	if err != nil {
		return TransferInfo{}, fmt.Errorf("stat remote file: %w", err)
	}
	if info.IsDir() {
		return TransferInfo{}, errors.New("download source must be a file")
	}
	if localInfo, err := os.Stat(localPath); err == nil && localInfo.IsDir() {
		localPath = filepath.Join(localPath, path.Base(remotePath))
	}
	if !request.Overwrite {
		if _, err := os.Stat(localPath); err == nil {
			return TransferInfo{}, fmt.Errorf("local file already exists: %s", localPath)
		}
	}

	transferID, ctx, cancel, err := m.registerTransfer(request.SessionID)
	if err != nil {
		return TransferInfo{}, err
	}

	go m.download(ctx, cancel, transferID, request.SessionID, localPath, remotePath, info.Size(), session.client)

	return TransferInfo{TransferID: transferID, Status: "started"}, nil
}

func (m *Manager) CancelTransfer(transferID string) error {
	m.mu.Lock()
	cancel, ok := m.transfers[transferID]
	if ok {
		delete(m.transfers, transferID)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("transfer not found: %s", transferID)
	}
	cancel()
	return nil
}

func (m *Manager) get(sessionID string) (*Session, error) {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sftp session not found: %s", sessionID)
	}
	return session, nil
}

func (m *Manager) registerTransfer(sessionID string) (string, context.Context, context.CancelFunc, error) {
	id, err := newID()
	if err != nil {
		return "", nil, nil, err
	}
	transferID := sessionID + ":" + id
	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	m.transfers[transferID] = cancel
	m.mu.Unlock()

	return transferID, ctx, cancel, nil
}

func (m *Manager) finishTransfer(transferID string, cancel context.CancelFunc) {
	cancel()
	m.mu.Lock()
	delete(m.transfers, transferID)
	m.mu.Unlock()
}

func (m *Manager) upload(ctx context.Context, cancel context.CancelFunc, transferID string, sessionID string, localPath string, remotePath string, total int64, client *sftp.Client) {
	defer m.finishTransfer(transferID, cancel)
	m.emit(ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "upload", LocalPath: localPath, RemotePath: remotePath, Total: total, Status: "started"})

	source, err := os.Open(localPath)
	if err != nil {
		m.emitFailure(ctx, transferID, sessionID, "upload", localPath, remotePath, total, 0, err)
		return
	}
	defer source.Close()

	target, err := client.OpenFile(remotePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		m.emitFailure(ctx, transferID, sessionID, "upload", localPath, remotePath, total, 0, err)
		return
	}
	defer target.Close()

	written, err := copyWithProgress(ctx, source, target, total, func(bytes int64) {
		m.emit(ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "upload", LocalPath: localPath, RemotePath: remotePath, Bytes: bytes, Total: total, Status: "in_progress"})
	})
	if err != nil {
		m.emitFailure(ctx, transferID, sessionID, "upload", localPath, remotePath, total, written, err)
		return
	}
	m.emit(ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "upload", LocalPath: localPath, RemotePath: remotePath, Bytes: written, Total: total, Status: "completed"})
}

func (m *Manager) download(ctx context.Context, cancel context.CancelFunc, transferID string, sessionID string, localPath string, remotePath string, total int64, client *sftp.Client) {
	defer m.finishTransfer(transferID, cancel)
	m.emit(ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "download", LocalPath: localPath, RemotePath: remotePath, Total: total, Status: "started"})

	source, err := client.Open(remotePath)
	if err != nil {
		m.emitFailure(ctx, transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		m.emitFailure(ctx, transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	target, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		m.emitFailure(ctx, transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	defer target.Close()

	written, err := copyWithProgress(ctx, source, target, total, func(bytes int64) {
		m.emit(ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "download", LocalPath: localPath, RemotePath: remotePath, Bytes: bytes, Total: total, Status: "in_progress"})
	})
	if err != nil {
		m.emitFailure(ctx, transferID, sessionID, "download", localPath, remotePath, total, written, err)
		return
	}
	m.emit(ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "download", LocalPath: localPath, RemotePath: remotePath, Bytes: written, Total: total, Status: "completed"})
}

func (m *Manager) emitFailure(ctx context.Context, transferID string, sessionID string, direction string, localPath string, remotePath string, total int64, bytes int64, err error) {
	status := "failed"
	message := err.Error()
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		status = "canceled"
		message = "transfer canceled"
	}
	m.emit(ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: direction, LocalPath: localPath, RemotePath: remotePath, Bytes: bytes, Total: total, Status: status, Message: message})
}

func (m *Manager) emit(event ProgressEvent) {
	select {
	case m.progress <- event:
	default:
	}
}

func copyWithProgress(ctx context.Context, source io.Reader, target io.Writer, total int64, progress func(bytes int64)) (int64, error) {
	buffer := make([]byte, 128*1024)
	var written int64
	var lastEmit int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}

		n, readErr := source.Read(buffer)
		if n > 0 {
			if err := ctx.Err(); err != nil {
				return written, err
			}
			w, writeErr := target.Write(buffer[:n])
			written += int64(w)
			if written-lastEmit >= 512*1024 || written == total {
				lastEmit = written
				progress(written)
			}
			if writeErr != nil {
				return written, writeErr
			}
			if w != n {
				return written, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			if written != lastEmit {
				progress(written)
			}
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

func cleanRemotePath(value string) (string, error) {
	if strings.ContainsRune(value, 0) {
		return "", errors.New("remote path contains invalid character")
	}
	if value == "" {
		return "/", nil
	}
	normalized := strings.ReplaceAll(value, "\\", "/")
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return "", errors.New("remote path traversal is not allowed")
		}
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	return path.Clean(normalized), nil
}

func CleanRemotePath(value string) (string, error) {
	return cleanRemotePath(value)
}

func cleanLocalPath(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("local path is required")
	}
	if strings.ContainsRune(value, 0) {
		return "", errors.New("local path contains invalid character")
	}
	cleaned := filepath.Clean(value)
	if !filepath.IsAbs(cleaned) {
		return "", errors.New("local path must be absolute")
	}
	return cleaned, nil
}

func CleanLocalPath(value string) (string, error) {
	return cleanLocalPath(value)
}

func newID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("create id: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}
