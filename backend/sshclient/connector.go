package sshclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lightssh/backend/config"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHConnector struct {
	KnownHostsPath  string
	HostKeyPrompter HostKeyPrompter
}

func NewSSHConnector() *SSHConnector {
	return &SSHConnector{}
}

type HostKeyPrompter interface {
	PromptHostKey(ctx context.Context, prompt HostKeyPrompt) (bool, error)
}

func (c *SSHConnector) Connect(ctx context.Context, request ConnectRequest) (TerminalSession, error) {
	client, err := c.DialClient(ctx, request)
	if err != nil {
		return nil, err
	}

	sshSession, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("create ssh session: %w", err)
	}

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return nil, fmt.Errorf("open ssh stdin: %w", err)
	}
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return nil, fmt.Errorf("open ssh stdout: %w", err)
	}
	stderr, err := sshSession.StderrPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return nil, fmt.Errorf("open ssh stderr: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sshSession.RequestPty("xterm-256color", request.Rows, request.Columns, modes); err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}
	if err := sshSession.Shell(); err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	sessionID, err := newSessionID()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return nil, err
	}

	session := &sshTerminalSession{
		id:      sessionID,
		client:  client,
		session: sshSession,
		stdin:   stdin,
		output:  make(chan OutputEvent, 64),
		done:    make(chan error, 1),
	}
	session.pipeOutput("stdout", stdout)
	session.pipeOutput("stderr", stderr)
	session.wait()

	return session, nil
}

func (c *SSHConnector) DialClient(ctx context.Context, request ConnectRequest) (*ssh.Client, error) {
	if err := request.Profile.Validate(); err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}

	timeout := time.Duration(request.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	authMethods, err := buildAuthMethods(request)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := c.hostKeyCallback(ctx)
	if err != nil {
		return nil, err
	}

	clientConfig := &ssh.ClientConfig{
		User:            request.Profile.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}

	address := net.JoinHostPort(request.Profile.Host, fmt.Sprintf("%d", request.Profile.Port))
	dialer := &net.Dialer{Timeout: timeout}
	netConn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, classifyConnectError(err)
	}

	conn, chans, reqs, err := ssh.NewClientConn(netConn, address, clientConfig)
	if err != nil {
		_ = netConn.Close()
		return nil, classifyConnectError(err)
	}

	return ssh.NewClient(conn, chans, reqs), nil
}

func (c *SSHConnector) hostKeyCallback(ctx context.Context) (ssh.HostKeyCallback, error) {
	path := c.KnownHostsPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user home for known_hosts: %w", err)
		}
		path = filepath.Join(home, ".ssh", "known_hosts")
	}

	callback, err := knownhosts.New(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	if errors.Is(err, os.ErrNotExist) {
		callback = func(string, net.Addr, ssh.PublicKey) error {
			return &knownhosts.KeyError{}
		}
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := callback(hostname, remote, key); err == nil {
			return nil
		} else {
			var keyErr *knownhosts.KeyError
			if !errors.As(err, &keyErr) {
				return err
			}
			if len(keyErr.Want) > 0 {
				return c.promptHostKey(ctx, path, hostname, remote, key, true)
			}
		}
		return c.promptHostKey(ctx, path, hostname, remote, key, false)
	}, nil
}

func (c *SSHConnector) promptHostKey(ctx context.Context, knownHostsPath string, hostname string, remote net.Addr, key ssh.PublicKey, changed bool) error {
	if c.HostKeyPrompter == nil {
		if changed {
			return fmt.Errorf("known host key changed for %s; presented %s", hostname, ssh.FingerprintSHA256(key))
		}
		return fmt.Errorf("unknown host key")
	}
	requestID, err := newSessionID()
	if err != nil {
		return err
	}
	accepted, err := c.HostKeyPrompter.PromptHostKey(ctx, HostKeyPrompt{
		RequestID:      requestID,
		Host:           hostname,
		Address:        remote.String(),
		KeyType:        key.Type(),
		Fingerprint:    ssh.FingerprintSHA256(key),
		KnownHosts:     knownHostsPath,
		KnownHostsLine: knownhosts.Line([]string{hostname}, key),
		Changed:        changed,
	})
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("host key rejected by user")
	}
	if err := appendKnownHost(knownHostsPath, hostname, key); err != nil {
		return err
	}
	return nil
}

func buildAuthMethods(request ConnectRequest) ([]ssh.AuthMethod, error) {
	switch request.Profile.AuthType {
	case config.AuthTypePassword:
		if request.Password == "" {
			return nil, fmt.Errorf("password is required")
		}
		return []ssh.AuthMethod{ssh.Password(request.Password)}, nil
	case config.AuthTypeKey:
		signer, err := privateKeySigner(request.Profile.KeyPath, request.Passphrase)
		if err != nil {
			return nil, err
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	case config.AuthTypeAgent:
		socket := os.Getenv("SSH_AUTH_SOCK")
		if socket == "" {
			return nil, fmt.Errorf("ssh-agent socket is not configured")
		}
		conn, err := net.Dial("unix", socket)
		if err != nil {
			return nil, fmt.Errorf("connect ssh-agent: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeysCallback(agent.NewClient(conn).Signers)}, nil
	default:
		return nil, fmt.Errorf("unsupported auth type: %q", request.Profile.AuthType)
	}
}

func privateKeySigner(keyPath string, passphrase string) (ssh.Signer, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("parse encrypted private key: %w", err)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return signer, nil
}

func appendKnownHost(path string, hostname string, key ssh.PublicKey) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create known_hosts directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts: %w", err)
	}
	defer file.Close()

	line := knownhosts.Line([]string{hostname}, key)
	if _, err := file.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}
	return nil
}

func classifyConnectError(err error) error {
	message := err.Error()
	switch {
	case strings.Contains(message, "unable to authenticate"):
		return fmt.Errorf("authentication failed: %w", err)
	case strings.Contains(message, "knownhosts:"):
		return fmt.Errorf("host key verification failed: %w", err)
	case strings.Contains(message, "i/o timeout"), strings.Contains(message, "deadline exceeded"):
		return fmt.Errorf("connection timed out: %w", err)
	default:
		return err
	}
}

func newSessionID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("create session id: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}

type sshTerminalSession struct {
	id      string
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	output  chan OutputEvent
	done    chan error

	pipeWG    sync.WaitGroup
	closeOnce sync.Once
}

func (s *sshTerminalSession) ID() string {
	return s.id
}

func (s *sshTerminalSession) Output() <-chan OutputEvent {
	return s.output
}

func (s *sshTerminalSession) Done() <-chan error {
	return s.done
}

func (s *sshTerminalSession) Write(data string) error {
	_, err := io.WriteString(s.stdin, data)
	return err
}

func (s *sshTerminalSession) KeepAlive() error {
	_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
	return err
}

func (s *sshTerminalSession) Resize(columns int, rows int) error {
	return s.session.WindowChange(rows, columns)
}

func (s *sshTerminalSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.session.Close()
		err = normalizeTerminalCloseError(err)
		if closeErr := s.client.Close(); err == nil {
			err = normalizeTerminalCloseError(closeErr)
		}
	})
	return err
}

func (s *sshTerminalSession) pipeOutput(stream string, reader io.Reader) {
	s.pipeWG.Add(1)
	go func() {
		defer s.pipeWG.Done()
		buffer := make([]byte, 4096)
		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				s.output <- OutputEvent{
					SessionID: s.id,
					Stream:    stream,
					Data:      string(buffer[:n]),
				}
			}
			if err != nil {
				return
			}
		}
	}()
}

func (s *sshTerminalSession) wait() {
	go func() {
		err := normalizeTerminalCloseError(s.session.Wait())
		_ = s.client.Close()
		s.pipeWG.Wait()
		close(s.output)
		s.done <- err
		close(s.done)
	}()
}

func normalizeTerminalCloseError(err error) error {
	if err == nil || err == io.EOF {
		return nil
	}
	var missingExit *ssh.ExitMissingError
	if errors.As(err, &missingExit) {
		return nil
	}
	if strings.Contains(err.Error(), "remote command exited without exit status or exit signal") {
		return nil
	}
	return err
}
