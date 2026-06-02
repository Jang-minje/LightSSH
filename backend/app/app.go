package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lightssh/backend/config"
	"lightssh/backend/security"
	"lightssh/backend/sftpclient"
	"lightssh/backend/sshclient"
	"lightssh/backend/tunnel"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx           context.Context
	store         *config.Store
	appearance    *config.AppearanceStore
	sshManager    *sshclient.Manager
	sftpManager   *sftpclient.Manager
	tunnelManager *tunnel.Manager

	hostKeyMu      sync.Mutex
	hostKeyPending map[string]sshclient.HostKeyPrompt

	cwdMu      sync.Mutex
	cwdPending map[string]*cwdRequest

	terminalMu      sync.Mutex
	terminalPending map[string]*terminalCommandRequest
}

type cwdRequest struct {
	token  string
	buffer strings.Builder
	result chan cwdResult
}

type cwdResult struct {
	path string
	err  error
}

func New() (*App, error) {
	store, err := config.NewDefaultStore()
	if err != nil {
		return nil, fmt.Errorf("create config store: %w", err)
	}
	appearanceStore, err := config.NewDefaultAppearanceStore()
	if err != nil {
		return nil, fmt.Errorf("create appearance store: %w", err)
	}

	sshConnector := sshclient.NewSSHConnector()
	application := &App{
		store:           store,
		appearance:      appearanceStore,
		sshManager:      sshclient.NewManager(sshConnector),
		sftpManager:     sftpclient.NewManager(sshConnector),
		tunnelManager:   tunnel.NewManager(tunnel.NewSSHConnectorDialer(sshConnector)),
		hostKeyPending:  make(map[string]sshclient.HostKeyPrompt),
		cwdPending:      make(map[string]*cwdRequest),
		terminalPending: make(map[string]*terminalCommandRequest),
	}
	sshConnector.HostKeyPrompter = application
	return application, nil
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	go a.relaySFTPProgress()
	go a.relayTunnelEvents()
}

func (a *App) Shutdown(_ context.Context) {
	a.sshManager.CloseAll()
	a.sftpManager.CloseAll()
	a.tunnelManager.StopAll()
}

func (a *App) AppName() string {
	return "LightSSH"
}

func (a *App) LoadProfiles() ([]config.ConnectionProfile, error) {
	profiles, err := a.store.LoadProfiles()
	if err != nil {
		return nil, err
	}
	for index := range profiles {
		profiles[index] = sanitizeProfile(profiles[index])
	}
	return profiles, nil
}

func (a *App) SaveProfiles(profiles []config.ConnectionProfile) error {
	return a.store.SaveProfiles(profiles)
}

func (a *App) LoadAppearanceSettings() (config.AppearanceSettings, error) {
	settings, err := a.appearance.Load()
	if err != nil {
		return config.AppearanceSettings{}, fmt.Errorf("%s", userMessage(err))
	}
	return settings, nil
}

func (a *App) SaveAppearanceSettings(settings config.AppearanceSettings) error {
	if err := a.appearance.Save(settings); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) SaveProfile(profile config.ConnectionProfile, password string, savePassword bool) (config.ConnectionProfile, error) {
	profiles, err := a.store.LoadProfiles()
	if err != nil {
		return config.ConnectionProfile{}, fmt.Errorf("%s", userMessage(err))
	}

	if strings.TrimSpace(profile.Name) == "" || profile.Name == "수동 접속" {
		profile.Name = profile.Username + "@" + profile.Host
	}

	var matched *config.ConnectionProfile
	for index := range profiles {
		if strings.EqualFold(strings.TrimSpace(profiles[index].Name), strings.TrimSpace(profile.Name)) {
			matched = &profiles[index]
			break
		}
	}

	if matched != nil {
		profile.ID = matched.ID
	} else {
		profile.ID, err = randomToken()
		if err != nil {
			return config.ConnectionProfile{}, fmt.Errorf("%s", userMessage(err))
		}
	}

	existingProtected := ""
	if matched != nil {
		existingProtected = matched.PasswordProtected
	}
	profile.PasswordProtected = ""
	profile.PasswordSaved = false
	if savePassword && profile.AuthType == config.AuthTypePassword {
		if password != "" {
			protected, err := security.ProtectString(password)
			if err != nil {
				return config.ConnectionProfile{}, fmt.Errorf("%s", userMessage(err))
			}
			profile.PasswordProtected = protected
		} else {
			profile.PasswordProtected = existingProtected
		}
		profile.PasswordSaved = profile.PasswordProtected != ""
	}

	next := make([]config.ConnectionProfile, 0, len(profiles)+1)
	for _, item := range profiles {
		if !strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(profile.Name)) {
			next = append(next, item)
		}
	}
	next = append(next, profile)
	if err := a.store.SaveProfiles(next); err != nil {
		return config.ConnectionProfile{}, fmt.Errorf("%s", userMessage(err))
	}
	return sanitizeProfile(profile), nil
}

func (a *App) DeleteProfile(profileID string) error {
	profiles, err := a.store.LoadProfiles()
	if err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	next := make([]config.ConnectionProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.ID != profileID {
			next = append(next, profile)
		}
	}
	if err := a.store.SaveProfiles(next); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) DefaultLocalDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%s", userMessage(err))
	}
	return home, nil
}

func (a *App) SelectLocalDirectory(current string) (string, error) {
	selected, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "저장 위치 선택",
		DefaultDirectory: current,
	})
	if err != nil {
		return "", fmt.Errorf("%s", userMessage(err))
	}
	return selected, nil
}

func (a *App) OpenLocalDirectory(localPath string) error {
	target := strings.TrimSpace(localPath)
	if target == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("%s", userMessage(err))
		}
		target = home
	}
	target = filepath.Clean(target)
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		target = filepath.Dir(target)
	}
	if err := exec.Command("explorer.exe", target).Start(); err != nil {
		return fmt.Errorf("%s", userMessage(fmt.Errorf("open explorer: %w", err)))
	}
	return nil
}

func (a *App) RevealLocalPath(localPath string) error {
	target := strings.TrimSpace(localPath)
	if target == "" {
		return a.OpenLocalDirectory("")
	}
	target = filepath.Clean(target)
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return a.OpenLocalDirectory(target)
	}
	if err := exec.Command("explorer.exe", "/select,"+target).Start(); err != nil {
		return fmt.Errorf("%s", userMessage(fmt.Errorf("reveal file: %w", err)))
	}
	return nil
}

func (a *App) PromptHostKey(ctx context.Context, prompt sshclient.HostKeyPrompt) (bool, error) {
	a.hostKeyMu.Lock()
	a.hostKeyPending[prompt.RequestID] = prompt
	a.hostKeyMu.Unlock()

	runtime.EventsEmit(a.ctx, "ssh:hostkey", prompt)
	return false, fmt.Errorf("host key approval required")
}

func (a *App) ResolveHostKeyPrompt(decision sshclient.HostKeyDecision) error {
	a.hostKeyMu.Lock()
	pending, ok := a.hostKeyPending[decision.RequestID]
	if ok {
		delete(a.hostKeyPending, decision.RequestID)
	}
	a.hostKeyMu.Unlock()
	if !ok {
		return fmt.Errorf("host key prompt not found")
	}
	if !decision.Accept {
		return nil
	}
	if pending.Changed {
		if err := replaceKnownHost(pending.KnownHosts, pending.Host, pending.KnownHostsLine); err != nil {
			return err
		}
		return nil
	}
	if err := appendKnownHostLine(pending.KnownHosts, pending.KnownHostsLine); err != nil {
		return err
	}
	return nil
}

func (a *App) StartSSHSession(request sshclient.ConnectRequest) (sshclient.SessionInfo, error) {
	request = a.hydratePassword(request)
	session, err := a.sshManager.Start(a.ctx, request)
	if err != nil {
		return sshclient.SessionInfo{}, fmt.Errorf("%s", userMessage(err))
	}

	go a.relaySSHEvents(session)

	return sshclient.SessionInfo{
		SessionID: session.ID(),
		Status:    "connected",
	}, nil
}

func (a *App) SendSSHInput(sessionID string, data string) error {
	if err := a.sshManager.Write(sessionID, data); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) KeepAliveSSHSession(sessionID string) error {
	if err := a.sshManager.KeepAlive(sessionID); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) ResizeSSHSession(sessionID string, columns int, rows int) error {
	if err := a.sshManager.Resize(sessionID, columns, rows); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) GetSSHWorkingDirectory(sessionID string) (string, error) {
	output, err := a.runTerminalCommand(sessionID, "printf '%s\\n' \"$PWD\"", 3*time.Second, 8192)
	if err != nil {
		return "", fmt.Errorf("%s", userMessage(err))
	}
	path, err := cleanWorkingDirectoryOutput(output)
	if err != nil {
		return "", fmt.Errorf("%s", userMessage(err))
	}
	return path, nil
}

func (a *App) DisconnectSSHSession(sessionID string) error {
	if err := a.sshManager.Close(sessionID); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) StartSFTPSession(request sftpclient.ConnectRequest) (sftpclient.SessionInfo, error) {
	request.SSH = a.hydratePassword(request.SSH)
	session, err := a.sftpManager.Start(a.ctx, request)
	if err != nil {
		return sftpclient.SessionInfo{}, fmt.Errorf("%s", userMessage(err))
	}
	return session, nil
}

func (a *App) CloseSFTPSession(sessionID string) error {
	if err := a.sftpManager.Close(sessionID); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) ListRemoteDirectory(sessionID string, remotePath string) ([]sftpclient.RemoteEntry, error) {
	entries, err := a.sftpManager.ListRemote(sessionID, remotePath)
	if err != nil {
		return nil, fmt.Errorf("%s", userMessage(err))
	}
	return entries, nil
}

func (a *App) ListLocalDirectory(localPath string) ([]sftpclient.LocalEntry, error) {
	entries, err := sftpclient.ListLocal(localPath)
	if err != nil {
		return nil, fmt.Errorf("%s", userMessage(err))
	}
	return entries, nil
}

func (a *App) UploadFile(request sftpclient.TransferRequest) (sftpclient.TransferInfo, error) {
	transfer, err := a.sftpManager.Upload(request)
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}
	return transfer, nil
}

func (a *App) DownloadFile(request sftpclient.TransferRequest) (sftpclient.TransferInfo, error) {
	transfer, err := a.sftpManager.Download(request)
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}
	return transfer, nil
}

func (a *App) CancelFileTransfer(transferID string) error {
	if err := a.sftpManager.CancelTransfer(transferID); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) MakeRemoteDirectory(sessionID string, remotePath string) error {
	if err := a.sftpManager.Mkdir(sessionID, remotePath); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) RenameRemotePath(sessionID string, oldPath string, newPath string) error {
	if err := a.sftpManager.Rename(sessionID, oldPath, newPath); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) DeleteRemotePath(sessionID string, remotePath string) error {
	if err := a.sftpManager.Delete(sessionID, remotePath); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) StartLocalTunnel(request tunnel.StartRequest) (tunnel.Info, error) {
	request.SSH = a.hydratePassword(request.SSH)
	info, err := a.tunnelManager.Start(a.ctx, request)
	if err != nil {
		return tunnel.Info{}, fmt.Errorf("%s", userMessage(err))
	}
	return info, nil
}

func (a *App) StartReverseTunnel(request tunnel.StartReverseRequest) (tunnel.Info, error) {
	request.SSH = a.hydratePassword(request.SSH)
	info, err := a.tunnelManager.StartReverse(a.ctx, request)
	if err != nil {
		return tunnel.Info{}, fmt.Errorf("%s", userMessage(err))
	}
	return info, nil
}

func (a *App) StopLocalTunnel(tunnelID string) error {
	if err := a.tunnelManager.Stop(tunnelID); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) ListTunnels() []tunnel.Info {
	return a.tunnelManager.List()
}

func (a *App) relaySFTPProgress() {
	for event := range a.sftpManager.Progress() {
		runtime.EventsEmit(a.ctx, "sftp:progress", event)
	}
}

func (a *App) relayTunnelEvents() {
	for event := range a.tunnelManager.Events() {
		runtime.EventsEmit(a.ctx, "tunnel:event", event)
	}
}

func (a *App) relaySSHEvents(session sshclient.TerminalSession) {
	for event := range session.Output() {
		if filtered, ok := a.filterTerminalCommandOutput(event.SessionID, event.Data); ok {
			event.Data = filtered
		} else {
			continue
		}
		runtime.EventsEmit(a.ctx, "ssh:data", event)
	}

	var message string
	if err, ok := <-session.Done(); ok && err != nil {
		message = userMessage(err)
		runtime.EventsEmit(a.ctx, "ssh:error", sshclient.CloseEvent{
			SessionID: session.ID(),
			Message:   message,
		})
	} else {
		message = "SSH 연결이 종료되었습니다."
	}
	a.sshManager.Forget(session.ID())
	runtime.EventsEmit(a.ctx, "ssh:closed", sshclient.CloseEvent{
		SessionID: session.ID(),
		Message:   message,
	})
}

func (a *App) hydratePassword(request sshclient.ConnectRequest) sshclient.ConnectRequest {
	if request.Profile.AuthType != config.AuthTypePassword || request.Password != "" || request.Profile.ID == "" {
		return request
	}
	profiles, err := a.store.LoadProfiles()
	if err != nil {
		return request
	}
	for _, profile := range profiles {
		if profile.ID != request.Profile.ID || profile.PasswordProtected == "" {
			continue
		}
		password, err := security.UnprotectString(profile.PasswordProtected)
		if err != nil {
			return request
		}
		request.Password = password
		return request
	}
	return request
}

func sanitizeProfile(profile config.ConnectionProfile) config.ConnectionProfile {
	profile.PasswordSaved = profile.PasswordProtected != ""
	profile.PasswordProtected = ""
	return profile
}

func (a *App) filterCWDOutput(sessionID string, data string) (string, bool) {
	a.cwdMu.Lock()
	request, ok := a.cwdPending[sessionID]
	if !ok {
		a.cwdMu.Unlock()
		return data, true
	}

	request.buffer.WriteString(data)
	text := request.buffer.String()
	startMarker := "__LIGHTSSH_PWD_" + request.token + "__"
	endMarker := "__LIGHTSSH_PWD_END_" + request.token + "__"
	start := findLineStartMarker(text, startMarker)
	if start >= 0 {
		pathStart := start + len(startMarker)
		if endOffset := strings.Index(text[pathStart:], endMarker); endOffset >= 0 {
			path := strings.TrimSpace(text[pathStart : pathStart+endOffset])
			tail := text[pathStart+endOffset+len(endMarker):]
			delete(a.cwdPending, sessionID)
			a.cwdMu.Unlock()
			request.result <- cwdResult{path: path}
			tail = strings.TrimLeft(tail, "\r\n")
			if tail == "" {
				return "", false
			}
			return tail, true
		}
	}
	if len(text) > 8192 {
		delete(a.cwdPending, sessionID)
		a.cwdMu.Unlock()
		request.result <- cwdResult{err: fmt.Errorf("terminal pwd marker not found")}
		return "", false
	}
	a.cwdMu.Unlock()
	return "", false
}

func (a *App) removeCWDRequest(sessionID string) {
	a.cwdMu.Lock()
	delete(a.cwdPending, sessionID)
	a.cwdMu.Unlock()
}

func findLineStartMarker(text string, marker string) int {
	index := strings.Index(text, marker)
	for index >= 0 {
		if index == 0 || text[index-1] == '\n' || text[index-1] == '\r' {
			return index
		}
		next := strings.Index(text[index+len(marker):], marker)
		if next < 0 {
			return -1
		}
		index = index + len(marker) + next
	}
	return -1
}

func userMessage(err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	switch {
	case strings.Contains(message, "authentication failed"), strings.Contains(message, "unable to authenticate"):
		return "인증에 실패했습니다. 사용자명, 비밀번호, 개인키 또는 ssh-agent 설정을 확인하세요."
	case strings.Contains(message, "connection timed out"), strings.Contains(message, "i/o timeout"), strings.Contains(message, "deadline exceeded"):
		return "SSH 연결 시간이 초과되었습니다. 호스트, 포트, 네트워크 상태를 확인하세요."
	case strings.Contains(message, "known host key changed"):
		return "저장된 호스트 키와 서버가 보낸 키가 다릅니다. 표시된 경고 모달에서 서버 교체 여부를 확인하세요."
	case strings.Contains(message, "host key verification failed"), strings.Contains(message, "known_hosts"), strings.Contains(message, "knownhosts"):
		return "호스트 키 검증에 실패했습니다. known_hosts 항목을 확인하세요."
	case strings.Contains(message, "host key approval required"):
		return "호스트 키 확인이 필요합니다. 표시된 모달에서 fingerprint를 확인하세요."
	case strings.Contains(message, "host key rejected by user"):
		return "호스트 키 승인이 취소되어 연결하지 않았습니다."
	case strings.Contains(message, "password is required"):
		return "비밀번호 인증을 선택한 경우 비밀번호를 입력해야 합니다."
	case strings.Contains(message, "ssh-agent socket is not configured"):
		return "ssh-agent가 설정되어 있지 않습니다."
	case strings.Contains(message, "remote forwarding is not allowed"):
		return "SSH 서버가 remote port forwarding을 허용하지 않습니다. 서버의 AllowTcpForwarding 설정을 확인하세요."
	case strings.Contains(message, "remote port already in use"):
		return "원격 포트가 이미 사용 중입니다. 다른 remote port를 선택하세요."
	default:
		return message
	}
}

func randomToken() (string, error) {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("create request token: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}

func appendKnownHostLine(path string, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create known_hosts directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}
	return nil
}

func replaceKnownHost(path string, host string, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create known_hosts directory: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read known_hosts: %w", err)
	}
	aliases := hostAliases(host)
	var kept []string
	for _, existing := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(existing) == "" {
			continue
		}
		if knownHostLineMatches(existing, aliases) {
			continue
		}
		kept = append(kept, existing)
	}
	kept = append(kept, line)
	next := strings.Join(kept, "\n") + "\n"
	if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}
	return nil
}

func knownHostLineMatches(line string, aliases map[string]struct{}) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
		return false
	}
	for _, marker := range strings.Split(fields[0], ",") {
		if _, ok := aliases[marker]; ok {
			return true
		}
	}
	return false
}

func hostAliases(host string) map[string]struct{} {
	aliases := map[string]struct{}{host: {}}
	if parsedHost, port, err := net.SplitHostPort(host); err == nil {
		aliases[parsedHost] = struct{}{}
		aliases["["+parsedHost+"]:"+port] = struct{}{}
	}
	return aliases
}
