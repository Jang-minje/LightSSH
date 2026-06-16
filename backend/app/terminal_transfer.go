package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"lightssh/backend/sftpclient"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type terminalCommandRequest struct {
	token       string
	startMarker string
	endMarker   string
	maxBytes    int
	buffer      strings.Builder
	result      chan terminalCommandResult
}

type terminalCommandResult struct {
	output string
	err    error
}

const terminalEntryPrefix = "__LIGHTSSH_ENTRY__\t"
const terminalSizePrefix = "__LIGHTSSH_SIZE__\t"
const terminalUserPrefix = "__LIGHTSSH_USER__\t"
const terminalUploadRawChunkSize = 768

func (a *App) GetSSHRemoteUser(sessionID string) (string, error) {
	output, err := a.runTerminalCommand(sessionID, "user=$(id -un 2>/dev/null || whoami 2>/dev/null)\nprintf '__LIGHTSSH_USER__\\t%s\\n' \"$user\"", 3*time.Second, 8192)
	if err != nil {
		return "", fmt.Errorf("%s", userMessage(err))
	}
	return cleanRemoteUserOutput(output), nil
}

func (a *App) ListRemoteDirectoryViaTerminal(sessionID string, remotePath string) ([]sftpclient.RemoteEntry, error) {
	cleanPath, err := sftpclient.CleanRemotePath(remotePath)
	if err != nil {
		return nil, fmt.Errorf("%s", userMessage(err))
	}

	command := fmt.Sprintf(
		"dir=%s\nif [ -d \"$dir\" ]; then find \"$dir\" -mindepth 1 -maxdepth 1 -printf '__LIGHTSSH_ENTRY__\\t%%f\\t%%p\\t%%y\\t%%s\\t%%M\\t%%TY-%%Tm-%%TdT%%TH:%%TM:%%TS%%TZ\\n' 2>/dev/null | cat; else printf 'ERROR\\tremote directory not found\\n'; fi",
		shellQuote(cleanPath),
	)
	output, err := a.runTerminalCommand(sessionID, command, 10*time.Second, 1024*1024)
	if err != nil {
		return nil, fmt.Errorf("%s", userMessage(err))
	}
	return parseTerminalRemoteEntries(output)
}

func (a *App) UploadFileViaTerminal(request sftpclient.TerminalTransferRequest) (sftpclient.TransferInfo, error) {
	localPath, err := sftpclient.CleanLocalPath(request.LocalPath)
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}
	remotePath, err := sftpclient.CleanRemotePath(request.RemotePath)
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(fmt.Errorf("stat local file: %w", err)))
	}
	if info.IsDir() {
		return sftpclient.TransferInfo{}, fmt.Errorf("upload source must be a file")
	}

	id, err := randomToken()
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}
	transferID := "terminal:" + id
	go a.uploadFileViaTerminal(request.SSHSessionID, transferID, localPath, remotePath, request.Overwrite, info.Size())
	return sftpclient.TransferInfo{TransferID: transferID, Status: "started"}, nil
}

func (a *App) DownloadFileViaTerminal(request sftpclient.TerminalTransferRequest) (sftpclient.TransferInfo, error) {
	remotePath, err := sftpclient.CleanRemotePath(request.RemotePath)
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}
	localPath, err := sftpclient.CleanLocalPath(request.LocalPath)
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}

	if localInfo, err := os.Stat(localPath); err == nil && localInfo.IsDir() {
		localPath = filepath.Join(localPath, path.Base(remotePath))
	}
	if !request.Overwrite {
		if _, err := os.Stat(localPath); err == nil {
			return sftpclient.TransferInfo{}, fmt.Errorf("local file already exists: %s", localPath)
		}
	}

	size, err := a.remoteFileSizeViaTerminal(request.SSHSessionID, remotePath)
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}

	id, err := randomToken()
	if err != nil {
		return sftpclient.TransferInfo{}, fmt.Errorf("%s", userMessage(err))
	}
	transferID := "terminal:" + id
	go a.downloadFileViaTerminal(request.SSHSessionID, transferID, localPath, remotePath, size)
	return sftpclient.TransferInfo{TransferID: transferID, Status: "started"}, nil
}

func (a *App) runTerminalCommand(sessionID string, body string, timeout time.Duration, maxBytes int) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	startMarker := "__LIGHTSSH_CMD_" + token + "__"
	endMarker := "__LIGHTSSH_CMD_END_" + token + "__"
	request := &terminalCommandRequest{
		token:       token,
		startMarker: startMarker,
		endMarker:   endMarker,
		maxBytes:    maxBytes,
		result:      make(chan terminalCommandResult, 1),
	}

	if err := a.registerTerminalRequest(sessionID, request); err != nil {
		return "", err
	}
	command := fmt.Sprintf(
		"%s%s\n%s\r",
		terminalCommandPrelude(startMarker),
		body,
		terminalCommandEpilogue(endMarker),
	)
	if err := a.sshManager.Write(sessionID, command); err != nil {
		a.removeTerminalRequest(sessionID)
		return "", err
	}

	return a.waitTerminalRequest(sessionID, request, timeout)
}

func (a *App) registerTerminalRequest(sessionID string, request *terminalCommandRequest) error {
	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	if _, exists := a.terminalPending[sessionID]; exists {
		return fmt.Errorf("이미 터미널 명령을 실행하는 중입니다.")
	}
	a.terminalPending[sessionID] = request
	return nil
}

func (a *App) waitTerminalRequest(sessionID string, request *terminalCommandRequest, timeout time.Duration) (string, error) {
	select {
	case result := <-request.result:
		if result.err != nil {
			return "", result.err
		}
		return result.output, nil
	case <-time.After(timeout):
		a.removeTerminalRequest(sessionID)
		_ = a.sshManager.Write(sessionID, terminalCommandRestore()+"\r")
		return "", fmt.Errorf("터미널 응답 시간이 초과되었습니다. 프롬프트 상태에서 다시 시도하세요.")
	}
}

func (a *App) filterTerminalCommandOutput(sessionID string, data string) (string, bool) {
	a.terminalMu.Lock()
	request, ok := a.terminalPending[sessionID]
	if !ok {
		a.terminalMu.Unlock()
		return data, true
	}

	request.buffer.WriteString(data)
	text := request.buffer.String()
	start := findLineStartMarker(text, request.startMarker)
	if start >= 0 {
		outputStart := start + len(request.startMarker)
		if endOffset := strings.Index(text[outputStart:], request.endMarker); endOffset >= 0 {
			output := strings.TrimSpace(text[outputStart : outputStart+endOffset])
			tail := text[outputStart+endOffset+len(request.endMarker):]
			delete(a.terminalPending, sessionID)
			a.terminalMu.Unlock()
			request.result <- terminalCommandResult{output: output}
			tail = strings.TrimLeft(tail, "\r\n")
			if tail == "" {
				return "", false
			}
			return tail, true
		}
	}
	if request.maxBytes > 0 && len(text) > request.maxBytes {
		delete(a.terminalPending, sessionID)
		a.terminalMu.Unlock()
		request.result <- terminalCommandResult{err: fmt.Errorf("terminal command output is too large")}
		return "", false
	}
	a.terminalMu.Unlock()
	return "", false
}

func (a *App) removeTerminalRequest(sessionID string) {
	a.terminalMu.Lock()
	delete(a.terminalPending, sessionID)
	a.terminalMu.Unlock()
}

func (a *App) uploadFileViaTerminal(sessionID string, transferID string, localPath string, remotePath string, overwrite bool, total int64) {
	a.emitSFTPProgress(sftpclient.ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "upload", LocalPath: localPath, RemotePath: remotePath, Total: total, Status: "started"})

	token, err := randomToken()
	if err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "upload", localPath, remotePath, total, 0, err)
		return
	}
	startMarker := "__LIGHTSSH_CMD_" + token + "__"
	endMarker := "__LIGHTSSH_CMD_END_" + token + "__"
	dataMarker := "__LIGHTSSH_UPLOAD_DATA_" + token + "__"
	request := &terminalCommandRequest{
		token:       token,
		startMarker: startMarker,
		endMarker:   endMarker,
		maxBytes:    64 * 1024,
		result:      make(chan terminalCommandResult, 1),
	}
	if err := a.registerTerminalRequest(sessionID, request); err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "upload", localPath, remotePath, total, 0, err)
		return
	}
	existsGuard := "false"
	if !overwrite {
		existsGuard = "true"
	}
	prefix := fmt.Sprintf(
		"%sremote=%s\nif [ %s = true ] && [ -e \"$remote\" ]; then printf 'ERROR\\tremote file already exists\\n'; else base64 -d > \"$remote\" <<'%s'\n",
		terminalCommandPrelude(startMarker),
		shellQuote(remotePath),
		existsGuard,
		dataMarker,
	)
	if err := a.sshManager.Write(sessionID, prefix); err != nil {
		a.removeTerminalRequest(sessionID)
		_ = a.sshManager.Write(sessionID, terminalCommandRestore()+"\r")
		a.emitTerminalTransferFailure(transferID, sessionID, "upload", localPath, remotePath, total, 0, err)
		return
	}

	written, writeErr := a.writeBase64FileToTerminal(sessionID, transferID, localPath, remotePath, total)
	suffix := terminalUploadSuffix(dataMarker, endMarker, total)
	if err := a.sshManager.Write(sessionID, suffix); writeErr == nil {
		writeErr = err
	}
	if writeErr != nil {
		a.removeTerminalRequest(sessionID)
		_ = a.sshManager.Write(sessionID, terminalCommandRestore()+"\r")
		a.emitTerminalTransferFailure(transferID, sessionID, "upload", localPath, remotePath, total, written, writeErr)
		return
	}

	output, err := a.waitTerminalRequest(sessionID, request, 30*time.Minute)
	if err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "upload", localPath, remotePath, total, written, err)
		return
	}
	if message, ok := terminalError(output); ok {
		a.emitTerminalTransferFailure(transferID, sessionID, "upload", localPath, remotePath, total, written, errors.New(message))
		return
	}
	a.emitSFTPProgress(sftpclient.ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "upload", LocalPath: localPath, RemotePath: remotePath, Bytes: total, Total: total, Status: "completed"})
}

func (a *App) writeBase64FileToTerminal(sessionID string, transferID string, localPath string, remotePath string, total int64) (int64, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	buffer := make([]byte, terminalUploadRawChunkSize)
	var written int64
	var lastEmit int64
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			chunk := terminalUploadBase64Line(buffer[:n])
			if err := a.sshManager.Write(sessionID, chunk); err != nil {
				return written, err
			}
			written += int64(n)
			if written-lastEmit >= 512*1024 || written == total {
				lastEmit = written
				a.emitSFTPProgress(sftpclient.ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "upload", LocalPath: localPath, RemotePath: remotePath, Bytes: written, Total: total, Status: "in_progress"})
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}

func terminalUploadBase64Line(data []byte) string {
	return base64.StdEncoding.EncodeToString(data) + "\n"
}

func terminalUploadSuffix(dataMarker string, endMarker string, total int64) string {
	expectedSize := shellQuote(fmt.Sprintf("%d", total))
	return fmt.Sprintf("\n%s\ncode=$?\nif [ \"$code\" -ne 0 ]; then printf 'ERROR\\tbase64 decode failed: %%s\\n' \"$code\"; else size=$(wc -c < \"$remote\" 2>/dev/null | tr -d '[:space:]'); if [ -z \"$size\" ]; then size=-1; fi; if [ \"$size\" != %s ]; then printf 'ERROR\\tremote file size mismatch: got %%s want %%s\\n' \"$size\" %s; fi; fi\nfi\n%s\r", dataMarker, expectedSize, expectedSize, terminalCommandEpilogue(endMarker))
}

func (a *App) remoteFileSizeViaTerminal(sessionID string, remotePath string) (int64, error) {
	command := fmt.Sprintf("remote=%s\nif [ -f \"$remote\" ]; then size=$(wc -c < \"$remote\"); printf '__LIGHTSSH_SIZE__\\t%%s\\n' \"$size\"; else printf 'ERROR\\tremote file not found\\n'; fi", shellQuote(remotePath))
	output, err := a.runTerminalCommand(sessionID, command, 10*time.Second, 64*1024)
	if err != nil {
		return 0, err
	}
	if message, ok := terminalError(output); ok {
		return 0, errors.New(message)
	}
	return parseTerminalFileSize(output)
}

func (a *App) downloadFileViaTerminal(sessionID string, transferID string, localPath string, remotePath string, total int64) {
	a.emitSFTPProgress(sftpclient.ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "download", LocalPath: localPath, RemotePath: remotePath, Total: total, Status: "started"})

	token, err := randomToken()
	if err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	dataStart := "__LIGHTSSH_B64_" + token + "__"
	dataEnd := "__LIGHTSSH_B64_END_" + token + "__"
	command := fmt.Sprintf("remote=%s\nif [ -f \"$remote\" ]; then printf '\\n%%s\\n' %s\nbase64 \"$remote\" | cat\nprintf '\\n%%s\\n' %s\nelse printf 'ERROR\\tremote file not found\\n'; fi", shellQuote(remotePath), shellQuote(dataStart), shellQuote(dataEnd))
	maxBytes := int(total*2 + 1024*1024)
	output, err := a.runTerminalCommand(sessionID, command, 30*time.Minute, maxBytes)
	if err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	if message, ok := terminalError(output); ok {
		a.emitTerminalTransferFailure(transferID, sessionID, "download", localPath, remotePath, total, 0, errors.New(message))
		return
	}
	cleaned, err := extractBase64Payload(output, dataStart, dataEnd)
	if err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	if err := os.WriteFile(localPath, data, 0o600); err != nil {
		a.emitTerminalTransferFailure(transferID, sessionID, "download", localPath, remotePath, total, 0, err)
		return
	}
	a.emitSFTPProgress(sftpclient.ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: "download", LocalPath: localPath, RemotePath: remotePath, Bytes: int64(len(data)), Total: total, Status: "completed"})
}

func (a *App) emitTerminalTransferFailure(transferID string, sessionID string, direction string, localPath string, remotePath string, total int64, bytes int64, err error) {
	a.emitSFTPProgress(sftpclient.ProgressEvent{TransferID: transferID, SessionID: sessionID, Direction: direction, LocalPath: localPath, RemotePath: remotePath, Bytes: bytes, Total: total, Status: "failed", Message: err.Error()})
}

func (a *App) emitSFTPProgress(event sftpclient.ProgressEvent) {
	runtime.EventsEmit(a.ctx, "sftp:progress", event)
}

func parseTerminalRemoteEntries(output string) ([]sftpclient.RemoteEntry, error) {
	output = stripTerminalControl(output)
	if message, ok := terminalError(output); ok {
		return nil, errors.New(message)
	}
	var entries []sftpclient.RemoteEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, terminalEntryPrefix) {
			continue
		}
		line = strings.TrimPrefix(line, terminalEntryPrefix)
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}
		if !strings.HasPrefix(parts[1], "/") || !isFindType(parts[2]) {
			continue
		}
		size, _ := strconv.ParseInt(parts[3], 10, 64)
		entries = append(entries, sftpclient.RemoteEntry{
			Name:    parts[0],
			Path:    parts[1],
			IsDir:   parts[2] == "d",
			Size:    size,
			Mode:    parts[4],
			ModTime: parts[5],
		})
	}
	return entries, nil
}

func isFindType(value string) bool {
	switch value {
	case "b", "c", "d", "f", "l", "p", "s":
		return true
	default:
		return false
	}
}

func terminalError(output string) (string, bool) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ERROR\t") {
			return strings.TrimSpace(strings.TrimPrefix(line, "ERROR\t")), true
		}
	}
	return "", false
}

func parseTerminalFileSize(output string) (int64, error) {
	output = stripTerminalControl(output)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if !strings.HasPrefix(line, terminalSizePrefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, terminalSizePrefix))
		size, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse remote file size: %w", err)
		}
		return size, nil
	}

	matches := numberPattern.FindAllString(output, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("parse remote file size: size marker not found")
	}
	size, err := strconv.ParseInt(matches[len(matches)-1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse remote file size: %w", err)
	}
	return size, nil
}

func extractBase64Payload(output string, startMarker string, endMarker string) (string, error) {
	output = stripTerminalControl(output)
	start := findLineStartMarker(output, startMarker)
	if start < 0 {
		return "", fmt.Errorf("download data marker not found")
	}
	payloadStart := start + len(startMarker)
	endOffset := strings.Index(output[payloadStart:], endMarker)
	if endOffset < 0 {
		return "", fmt.Errorf("download data end marker not found")
	}
	payload := output[payloadStart : payloadStart+endOffset]
	var builder strings.Builder
	for _, r := range payload {
		if (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '+' ||
			r == '/' ||
			r == '=' {
			builder.WriteRune(r)
		}
	}
	cleaned := builder.String()
	if cleaned == "" {
		return "", fmt.Errorf("download data is empty")
	}
	return cleaned, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func terminalCommandPrelude(startMarker string) string {
	return fmt.Sprintf(
		" __LIGHTSSH_HIST_START=${HISTCMD:-0}\n __LIGHTSSH_OLD_HISTCONTROL=${HISTCONTROL-}\n HISTCONTROL=ignorespace${HISTCONTROL:+:$HISTCONTROL}\n set +o history 2>/dev/null || true\n stty -echo 2>/dev/null || true\n printf '\\n%%s\\n' %s\n",
		shellQuote(startMarker),
	)
}

func terminalCommandEpilogue(endMarker string) string {
	return fmt.Sprintf(
		"printf '\\n%%s\\n' %s\n%s",
		shellQuote(endMarker),
		terminalCommandRestore(),
	)
}

func terminalCommandRestore() string {
	return ` case ${HISTCMD:-} in
''|*[!0-9]*) ;;
*)
  case ${__LIGHTSSH_HIST_START:-} in
  ''|*[!0-9]*|0) ;;
  *)
    __LIGHTSSH_HIST_CURSOR=${HISTCMD:-0}
    while [ "$__LIGHTSSH_HIST_CURSOR" -gt "$__LIGHTSSH_HIST_START" ] 2>/dev/null; do
      history -d "$((__LIGHTSSH_HIST_CURSOR - 1))" 2>/dev/null || break
      __LIGHTSSH_HIST_CURSOR=$((__LIGHTSSH_HIST_CURSOR - 1))
    done
    history -d "$__LIGHTSSH_HIST_START" 2>/dev/null || true
    ;;
  esac
  ;;
esac
 if [ "${__LIGHTSSH_OLD_HISTCONTROL+x}" = x ]; then HISTCONTROL=$__LIGHTSSH_OLD_HISTCONTROL; else unset HISTCONTROL; fi
 unset __LIGHTSSH_HIST_START __LIGHTSSH_HIST_CURSOR __LIGHTSSH_OLD_HISTCONTROL
 set -o history 2>/dev/null || true
 stty echo 2>/dev/null || true`
}

var unixPathPattern = regexp.MustCompile(`(?:^|[\s"'])(/[A-Za-z0-9._~+\-/%:@=,]+)`)
var numberPattern = regexp.MustCompile(`\b\d+\b`)

func cleanWorkingDirectoryOutput(output string) (string, error) {
	output = stripTerminalControl(output)
	lines := strings.Split(output, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(strings.TrimRight(lines[index], "\r"))
		if strings.HasPrefix(line, "/") {
			return path.Clean(line), nil
		}
	}
	matches := unixPathPattern.FindAllStringSubmatch(output, -1)
	if len(matches) > 0 {
		return path.Clean(matches[len(matches)-1][1]), nil
	}
	return "", fmt.Errorf("terminal pwd marker did not contain a path")
}

func cleanSingleLineOutput(output string) string {
	output = stripTerminalControl(output)
	lines := strings.Split(output, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(strings.TrimRight(lines[index], "\r"))
		if line != "" {
			return line
		}
	}
	return ""
}

func cleanRemoteUserOutput(output string) string {
	output = stripTerminalControl(output)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if !strings.HasPrefix(line, terminalUserPrefix) {
			continue
		}
		user := strings.TrimSpace(strings.TrimPrefix(line, terminalUserPrefix))
		if isShellUsername(user) {
			return user
		}
	}
	return cleanSingleLineOutput(output)
}

func isShellUsername(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' ||
			r == '-' ||
			r == '.' {
			continue
		}
		return false
	}
	return true
}

func stripTerminalControl(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] == 0x1b {
			index++
			if index < len(value) && value[index] == ']' {
				for index < len(value) && value[index] != 0x07 {
					if value[index] == 0x1b && index+1 < len(value) && value[index+1] == '\\' {
						index++
						break
					}
					index++
				}
				continue
			}
			for index < len(value) && (value[index] < '@' || value[index] > '~') {
				index++
			}
			continue
		}
		builder.WriteByte(value[index])
	}
	return builder.String()
}
