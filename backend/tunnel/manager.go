package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type Manager struct {
	dialer SSHDialer

	mu      sync.RWMutex
	tunnels map[string]*localTunnel
	reverse map[string]*reverseTunnel
	events  chan Event
}

type localTunnel struct {
	config   LocalConfig
	dialer   TargetDialer
	listener net.Listener
	cancel   context.CancelFunc
	done     chan struct{}

	mu    sync.Mutex
	conns map[net.Conn]bool
	count atomic.Int32
}

type reverseTunnel struct {
	config   ReverseConfig
	dialer   TargetDialer
	listener net.Listener
	cancel   context.CancelFunc
	done     chan struct{}

	mu    sync.Mutex
	conns map[net.Conn]bool
	count atomic.Int32
}

func NewManager(dialer SSHDialer) *Manager {
	return &Manager{
		dialer:  dialer,
		tunnels: make(map[string]*localTunnel),
		reverse: make(map[string]*reverseTunnel),
		events:  make(chan Event, 128),
	}
}

func (m *Manager) Events() <-chan Event {
	return m.events
}

func (m *Manager) Start(ctx context.Context, request StartRequest) (Info, error) {
	if err := request.Config.Validate(); err != nil {
		return Info{}, fmt.Errorf("invalid tunnel config: %w", err)
	}
	if err := request.SSH.Profile.Validate(); err != nil {
		return Info{}, fmt.Errorf("invalid ssh profile: %w", err)
	}

	m.mu.RLock()
	if _, exists := m.tunnels[request.Config.ID]; exists {
		m.mu.RUnlock()
		return Info{}, fmt.Errorf("tunnel already running: %s", request.Config.ID)
	}
	if _, exists := m.reverse[request.Config.ID]; exists {
		m.mu.RUnlock()
		return Info{}, fmt.Errorf("tunnel already running: %s", request.Config.ID)
	}
	for _, tunnel := range m.tunnels {
		if tunnel.config.LocalBindAddress == request.Config.LocalBindAddress && tunnel.config.LocalPort == request.Config.LocalPort {
			m.mu.RUnlock()
			return Info{}, fmt.Errorf("local port already used by tunnel: %s:%d", request.Config.LocalBindAddress, request.Config.LocalPort)
		}
	}
	m.mu.RUnlock()

	targetDialer, err := m.dialer.Dial(ctx, request.SSH)
	if err != nil {
		return Info{}, fmt.Errorf("connect ssh for tunnel: %w", err)
	}

	listenAddress := net.JoinHostPort(request.Config.LocalBindAddress, strconv.Itoa(request.Config.LocalPort))
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		_ = targetDialer.Close()
		return Info{}, fmt.Errorf("listen on local tunnel address: %w", err)
	}

	config := request.Config
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		config.LocalPort = tcpAddr.Port
	}
	tunnelCtx, cancel := context.WithCancel(context.Background())
	instance := &localTunnel{
		config:   config,
		dialer:   targetDialer,
		listener: listener,
		cancel:   cancel,
		done:     make(chan struct{}),
		conns:    make(map[net.Conn]bool),
	}

	m.mu.Lock()
	m.tunnels[config.ID] = instance
	m.mu.Unlock()

	go m.acceptLoop(tunnelCtx, instance)
	m.emit(Event{TunnelID: config.ID, Status: "running", ConnectionCount: 0})

	return instance.info("running", ""), nil
}

func (m *Manager) StartReverse(ctx context.Context, request StartReverseRequest) (Info, error) {
	if err := request.Config.Validate(); err != nil {
		return Info{}, fmt.Errorf("invalid reverse tunnel config: %w", err)
	}
	if err := request.SSH.Profile.Validate(); err != nil {
		return Info{}, fmt.Errorf("invalid ssh profile: %w", err)
	}

	m.mu.RLock()
	if _, exists := m.tunnels[request.Config.ID]; exists {
		m.mu.RUnlock()
		return Info{}, fmt.Errorf("tunnel already running: %s", request.Config.ID)
	}
	if _, exists := m.reverse[request.Config.ID]; exists {
		m.mu.RUnlock()
		return Info{}, fmt.Errorf("tunnel already running: %s", request.Config.ID)
	}
	for _, tunnel := range m.reverse {
		if tunnel.config.RemoteBindAddress == request.Config.RemoteBindAddress && tunnel.config.RemotePort == request.Config.RemotePort {
			m.mu.RUnlock()
			return Info{}, fmt.Errorf("remote port already used by tunnel: %s:%d", request.Config.RemoteBindAddress, request.Config.RemotePort)
		}
	}
	m.mu.RUnlock()

	targetDialer, err := m.dialer.Dial(ctx, request.SSH)
	if err != nil {
		return Info{}, fmt.Errorf("connect ssh for reverse tunnel: %w", err)
	}

	listenAddress := net.JoinHostPort(request.Config.RemoteBindAddress, strconv.Itoa(request.Config.RemotePort))
	listener, err := targetDialer.Listen("tcp", listenAddress)
	if err != nil {
		_ = targetDialer.Close()
		return Info{}, classifyReverseListenError(err)
	}

	tunnelCtx, cancel := context.WithCancel(context.Background())
	instance := &reverseTunnel{
		config:   request.Config,
		dialer:   targetDialer,
		listener: listener,
		cancel:   cancel,
		done:     make(chan struct{}),
		conns:    make(map[net.Conn]bool),
	}

	m.mu.Lock()
	m.reverse[request.Config.ID] = instance
	m.mu.Unlock()

	go m.acceptReverseLoop(tunnelCtx, instance)
	m.emit(Event{TunnelID: request.Config.ID, Status: "running", ConnectionCount: 0})

	return instance.info("running", ""), nil
}

func (m *Manager) Stop(tunnelID string) error {
	m.mu.Lock()
	tunnel, ok := m.tunnels[tunnelID]
	if ok {
		delete(m.tunnels, tunnelID)
	}
	reverseTunnel, reverseOK := m.reverse[tunnelID]
	if reverseOK {
		delete(m.reverse, tunnelID)
	}
	m.mu.Unlock()
	if ok {
		tunnel.cancel()
		_ = tunnel.listener.Close()
		tunnel.closeConnections()
		_ = tunnel.dialer.Close()
		<-tunnel.done
		m.emit(Event{TunnelID: tunnelID, Status: "stopped", ConnectionCount: 0})
		return nil
	}
	if reverseOK {
		reverseTunnel.cancel()
		_ = reverseTunnel.listener.Close()
		reverseTunnel.closeConnections()
		_ = reverseTunnel.dialer.Close()
		<-reverseTunnel.done
		m.emit(Event{TunnelID: tunnelID, Status: "stopped", ConnectionCount: 0})
		return nil
	}
	if !ok && !reverseOK {
		return fmt.Errorf("tunnel not found: %s", tunnelID)
	}
	return nil
}

func (m *Manager) StopAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.tunnels))
	for id := range m.tunnels {
		ids = append(ids, id)
	}
	for id := range m.reverse {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		_ = m.Stop(id)
	}
}

func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Info, 0, len(m.tunnels))
	for _, tunnel := range m.tunnels {
		result = append(result, tunnel.info("running", ""))
	}
	for _, tunnel := range m.reverse {
		result = append(result, tunnel.info("running", ""))
	}
	return result
}

func (m *Manager) acceptLoop(ctx context.Context, tunnel *localTunnel) {
	defer close(tunnel.done)
	for {
		localConn, err := tunnel.listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			m.emit(Event{TunnelID: tunnel.config.ID, Status: "error", ConnectionCount: tunnel.connectionCount(), Message: sanitizeError(err)})
			continue
		}
		tunnel.addConn(localConn, true)
		m.emit(Event{TunnelID: tunnel.config.ID, Status: "running", ConnectionCount: tunnel.connectionCount()})
		go m.forward(ctx, tunnel, localConn)
	}
}

func (m *Manager) acceptReverseLoop(ctx context.Context, tunnel *reverseTunnel) {
	defer close(tunnel.done)
	for {
		remoteConn, err := tunnel.listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			m.emit(Event{TunnelID: tunnel.config.ID, Status: "error", ConnectionCount: tunnel.connectionCount(), Message: sanitizeError(err)})
			continue
		}
		tunnel.addConn(remoteConn, true)
		m.emit(Event{TunnelID: tunnel.config.ID, Status: "running", ConnectionCount: tunnel.connectionCount()})
		go m.forwardReverse(ctx, tunnel, remoteConn)
	}
}

func (m *Manager) forwardReverse(ctx context.Context, tunnel *reverseTunnel, remoteConn net.Conn) {
	defer func() {
		tunnel.removeConn(remoteConn)
		_ = remoteConn.Close()
		m.emit(Event{TunnelID: tunnel.config.ID, Status: "running", ConnectionCount: tunnel.connectionCount()})
	}()

	localAddress := net.JoinHostPort(tunnel.config.LocalTargetHost, strconv.Itoa(tunnel.config.LocalTargetPort))
	localConn, err := net.Dial("tcp", localAddress)
	if err != nil {
		m.emit(Event{TunnelID: tunnel.config.ID, Status: "error", ConnectionCount: tunnel.connectionCount(), Message: sanitizeError(err)})
		return
	}
	tunnel.addConn(localConn, false)
	defer func() {
		tunnel.removeConn(localConn)
		_ = localConn.Close()
	}()

	done := make(chan struct{}, 2)
	go proxy(remoteConn, localConn, done)
	go proxy(localConn, remoteConn, done)

	select {
	case <-ctx.Done():
	case <-done:
	}
}

func (m *Manager) forward(ctx context.Context, tunnel *localTunnel, localConn net.Conn) {
	defer func() {
		tunnel.removeConn(localConn)
		_ = localConn.Close()
		m.emit(Event{TunnelID: tunnel.config.ID, Status: "running", ConnectionCount: tunnel.connectionCount()})
	}()

	targetAddress := net.JoinHostPort(tunnel.config.TargetHost, strconv.Itoa(tunnel.config.TargetPort))
	remoteConn, err := tunnel.dialer.Dial("tcp", targetAddress)
	if err != nil {
		m.emit(Event{TunnelID: tunnel.config.ID, Status: "error", ConnectionCount: tunnel.connectionCount(), Message: sanitizeError(err)})
		return
	}
	tunnel.addConn(remoteConn, false)
	defer func() {
		tunnel.removeConn(remoteConn)
		_ = remoteConn.Close()
	}()

	done := make(chan struct{}, 2)
	go proxy(localConn, remoteConn, done)
	go proxy(remoteConn, localConn, done)

	select {
	case <-ctx.Done():
	case <-done:
	}
}

func proxy(dst net.Conn, src net.Conn, done chan<- struct{}) {
	_, _ = io.Copy(dst, src)
	_ = dst.Close()
	_ = src.Close()
	done <- struct{}{}
}

func (m *Manager) emit(event Event) {
	select {
	case m.events <- event:
	default:
	}
}

func (t *localTunnel) info(status string, message string) Info {
	return Info{
		TunnelID:        t.config.ID,
		Direction:       "local",
		Name:            t.config.Name,
		LocalAddress:    t.config.LocalBindAddress,
		LocalPort:       t.config.LocalPort,
		TargetHost:      t.config.TargetHost,
		TargetPort:      t.config.TargetPort,
		Status:          status,
		ConnectionCount: t.connectionCount(),
		Message:         message,
	}
}

func (t *reverseTunnel) info(status string, message string) Info {
	return Info{
		TunnelID:        t.config.ID,
		Direction:       "reverse",
		Name:            t.config.Name,
		RemoteAddress:   t.config.RemoteBindAddress,
		RemotePort:      t.config.RemotePort,
		TargetHost:      t.config.LocalTargetHost,
		TargetPort:      t.config.LocalTargetPort,
		Status:          status,
		ConnectionCount: t.connectionCount(),
		Message:         message,
	}
}

func (t *localTunnel) addConn(conn net.Conn, counted bool) {
	t.mu.Lock()
	t.conns[conn] = counted
	t.mu.Unlock()
	if counted {
		t.count.Add(1)
	}
}

func (t *localTunnel) removeConn(conn net.Conn) {
	t.mu.Lock()
	if counted, ok := t.conns[conn]; ok {
		delete(t.conns, conn)
		if counted {
			t.count.Add(-1)
		}
	}
	t.mu.Unlock()
}

func (t *localTunnel) closeConnections() {
	t.mu.Lock()
	conns := make([]net.Conn, 0, len(t.conns))
	for conn := range t.conns {
		conns = append(conns, conn)
	}
	t.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

func (t *localTunnel) connectionCount() int {
	count := t.count.Load()
	if count < 0 {
		return 0
	}
	return int(count)
}

func (t *reverseTunnel) addConn(conn net.Conn, counted bool) {
	t.mu.Lock()
	t.conns[conn] = counted
	t.mu.Unlock()
	if counted {
		t.count.Add(1)
	}
}

func (t *reverseTunnel) removeConn(conn net.Conn) {
	t.mu.Lock()
	if counted, ok := t.conns[conn]; ok {
		delete(t.conns, conn)
		if counted {
			t.count.Add(-1)
		}
	}
	t.mu.Unlock()
}

func (t *reverseTunnel) closeConnections() {
	t.mu.Lock()
	conns := make([]net.Conn, 0, len(t.conns))
	for conn := range t.conns {
		conns = append(conns, conn)
	}
	t.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

func (t *reverseTunnel) connectionCount() int {
	count := t.count.Load()
	if count < 0 {
		return 0
	}
	return int(count)
}

func classifyReverseListenError(err error) error {
	message := err.Error()
	switch {
	case strings.Contains(message, "administratively prohibited"), strings.Contains(message, "tcpip-forward"), strings.Contains(message, "remote port forwarding failed"):
		return fmt.Errorf("remote forwarding is not allowed by the SSH server: %w", err)
	case strings.Contains(message, "address already in use"), strings.Contains(message, "bind: address already in use"):
		return fmt.Errorf("remote port already in use: %w", err)
	default:
		return fmt.Errorf("listen on remote tunnel address: %w", err)
	}
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
