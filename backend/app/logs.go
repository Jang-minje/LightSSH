package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var appLogMu sync.Mutex

func (a *App) AppendAppLog(line string) error {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return nil
	}
	path, err := appLogPath()
	if err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	appLogMu.Lock()
	defer appLogMu.Unlock()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	defer file.Close()
	if _, err := file.WriteString(sanitizeAppLogLine(line) + "\n"); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	return nil
}

func (a *App) RevealAppLogFile() error {
	path, err := appLogPath()
	if err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("%s", userMessage(err))
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte{}, 0600); err != nil {
			return fmt.Errorf("%s", userMessage(err))
		}
	}
	if err := exec.Command("explorer.exe", "/select,"+path).Start(); err != nil {
		return fmt.Errorf("%s", userMessage(fmt.Errorf("reveal app log: %w", err)))
	}
	return nil
}

func appLogPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	fileName := "lightssh-" + time.Now().Format("2006-01-02") + ".log"
	return filepath.Join(dir, "LightSSH", "logs", fileName), nil
}

func sanitizeAppLogLine(line string) string {
	line = strings.ReplaceAll(line, "\x00", "")
	line = strings.ReplaceAll(line, "\r", " ")
	return strings.TrimSpace(line)
}
