package sshclient

import (
	"errors"
	"io"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNormalizeTerminalCloseErrorIgnoresExpectedCloseErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "nil", err: nil},
		{name: "eof", err: io.EOF},
		{name: "missing exit status", err: &ssh.ExitMissingError{}},
		{name: "wrapped missing exit status", err: errors.New("wait: remote command exited without exit status or exit signal")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := normalizeTerminalCloseError(tt.err); err != nil {
				t.Fatalf("normalizeTerminalCloseError() = %v, want nil", err)
			}
		})
	}
}

func TestNormalizeTerminalCloseErrorKeepsUnexpectedErrors(t *testing.T) {
	err := errors.New("network reset")
	if got := normalizeTerminalCloseError(err); got == nil {
		t.Fatal("normalizeTerminalCloseError() = nil, want original error")
	}
}
