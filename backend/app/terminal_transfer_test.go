package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseTerminalRemoteEntries(t *testing.T) {
	output := terminalEntryPrefix + "file.txt\t/home/dev/file.txt\tf\t123\t-rw-r--r--\t2026-05-18T10:11:12.0000000000\n" +
		terminalEntryPrefix + "공개\t/home/dev/공개\td\t0\tdrwxr-xr-x\t2026-05-18T10:12:12.0000000000\n"

	entries, err := parseTerminalRemoteEntries(output)
	if err != nil {
		t.Fatalf("parse entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].Name != "file.txt" || entries[0].IsDir || entries[0].Size != 123 {
		t.Fatalf("unexpected file entry: %+v", entries[0])
	}
	if entries[1].Name != "공개" || !entries[1].IsDir {
		t.Fatalf("unexpected directory entry: %+v", entries[1])
	}
}

func TestParseTerminalRemoteEntriesSkipsEchoedCommand(t *testing.T) {
	output := "[root@host ~]# printf 'name\\tpath\\ttype\\tsize\\tmode\\ttime\\n'\n" +
		"\x1b]0;root@localhost:~\x07[root@host ~]# find /root -printf '__LIGHTSSH_ENTRY__\\t%f\\t%p\\t%y\\t%s\\t%M\\t%T+\\n'\n" +
		terminalEntryPrefix + "file.txt\t/root/file.txt\tf\t18\t-rw-r--r--\t2026-05-18T10:11:12.0000000000\n"
	entries, err := parseTerminalRemoteEntries(output)
	if err != nil {
		t.Fatalf("parse entries: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "file.txt" {
		t.Fatalf("entries = %+v, want one file entry", entries)
	}
}

func TestParseTerminalRemoteEntriesError(t *testing.T) {
	_, err := parseTerminalRemoteEntries("ERROR\tremote directory not found\n")
	if err == nil {
		t.Fatal("expected terminal error")
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("/tmp/a'b")
	want := "'/tmp/a'\"'\"'b'"
	if got != want {
		t.Fatalf("quote = %q, want %q", got, want)
	}
}

func TestTerminalCommandWrapperDisablesHistory(t *testing.T) {
	command := terminalCommandPrelude("__LIGHTSSH_CMD_test__") +
		"pwd\n" +
		terminalCommandEpilogue("__LIGHTSSH_CMD_END_test__")

	required := []string{
		"__LIGHTSSH_HIST_START=${HISTCMD:-0}",
		"HISTCONTROL=ignorespace",
		"set +o history",
		"stty -echo",
		"__LIGHTSSH_CMD_test__",
		"__LIGHTSSH_CMD_END_test__",
		"history -d",
		"__LIGHTSSH_OLD_HISTCONTROL",
		"set -o history",
		"stty echo",
	}
	for _, part := range required {
		if !strings.Contains(command, part) {
			t.Fatalf("wrapper does not contain %q:\n%s", part, command)
		}
	}
}

func TestCleanWorkingDirectoryOutput(t *testing.T) {
	output := "\x1b]0;root@localhost:~\x07[root@localhost ~]# printf '%s\\n' \"$PWD\"\r\n/root\r\n"
	got, err := cleanWorkingDirectoryOutput(output)
	if err != nil {
		t.Fatalf("clean pwd: %v", err)
	}
	if got != "/root" {
		t.Fatalf("path = %q, want /root", got)
	}
}

func TestCleanSingleLineOutput(t *testing.T) {
	got := cleanSingleLineOutput("\x1b]0;root@host:~\x07\r\nroot\r\n")
	if got != "root" {
		t.Fatalf("line = %q, want root", got)
	}
}

func TestCleanRemoteUserOutputUsesMarker(t *testing.T) {
	output := "\x1b]0;root@KAITEB:/home/sinja03\x07[root@KAITEB sinja03]# id -un\r\n" +
		terminalUserPrefix + "root\r\n" +
		"[root@KAITEB sinja03]#"
	got := cleanRemoteUserOutput(output)
	if got != "root" {
		t.Fatalf("user = %q, want root", got)
	}
}

func TestCleanRemoteUserOutputRejectsPromptAsUser(t *testing.T) {
	output := terminalUserPrefix + "[root@KAITEB sinja03]#\nroot\n"
	got := cleanRemoteUserOutput(output)
	if got != "root" {
		t.Fatalf("user = %q, want root fallback", got)
	}
}

func TestParseTerminalFileSize(t *testing.T) {
	output := "\x1b]0;root@localhost:~\x07[root@localhost ~]# " +
		terminalSizePrefix + "11247199\r\n" +
		"\x1b]0;root@localhost:~\x07[root@localhost ~]#"
	got, err := parseTerminalFileSize(output)
	if err != nil {
		t.Fatalf("parse size: %v", err)
	}
	if got != 11247199 {
		t.Fatalf("size = %d, want 11247199", got)
	}
}

func TestParseTerminalFileSizeFallback(t *testing.T) {
	output := "\x1b]0;root@localhost:~\x07[root@localhost ~]# \x1b]0;root@localhost:~\x07[root@localhost ~]# 11247199\r\n\x1b]0;root@localhost:~\x07[root@localhost ~]#"
	got, err := parseTerminalFileSize(output)
	if err != nil {
		t.Fatalf("parse size fallback: %v", err)
	}
	if got != 11247199 {
		t.Fatalf("size = %d, want 11247199", got)
	}
}

func TestExtractBase64PayloadSkipsPromptAndEcho(t *testing.T) {
	start := "__LIGHTSSH_B64_test__"
	end := "__LIGHTSSH_B64_END_test__"
	output := "\x1b]0;root@localhost:~\x07[root@localhost ~]# base64 file\n" +
		start + "\n" +
		"SGVs\nbG8=\n" +
		end + "\n" +
		"\x1b]0;root@localhost:~\x07[root@localhost ~]#"
	got, err := extractBase64Payload(output, start, end)
	if err != nil {
		t.Fatalf("extract payload: %v", err)
	}
	if got != "SGVsbG8=" {
		t.Fatalf("payload = %q, want SGVsbG8=", got)
	}
}

func TestExtractBase64PayloadRejectsMissingMarker(t *testing.T) {
	if _, err := extractBase64Payload("SGVsbG8=", "start", "end"); err == nil {
		t.Fatal("expected marker error")
	}
}

func TestTerminalUploadBase64LineFitsPTYCanonicalInput(t *testing.T) {
	data := bytes.Repeat([]byte{0xff}, terminalUploadRawChunkSize)
	line := terminalUploadBase64Line(data)
	if !strings.HasSuffix(line, "\n") {
		t.Fatal("line must end with newline")
	}
	encoded := strings.TrimSuffix(line, "\n")
	if len(encoded) > 1024 {
		t.Fatalf("encoded line length = %d, want <= 1024", len(encoded))
	}
}

func TestTerminalUploadSuffixChecksRemoteFileSize(t *testing.T) {
	suffix := terminalUploadSuffix("__DATA__", "__END__", 123)
	required := []string{
		"wc -c",
		"tr -d '[:space:]'",
		"remote file size mismatch",
		"want %s",
		"'123'",
	}
	for _, part := range required {
		if !strings.Contains(suffix, part) {
			t.Fatalf("suffix does not contain %q:\n%s", part, suffix)
		}
	}
	if strings.Contains(suffix, "want '123'") {
		t.Fatalf("suffix embeds quoted size inside printf format:\n%s", suffix)
	}
}
