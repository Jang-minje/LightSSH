package sftpclient

import "testing"

func TestCleanRemotePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty becomes root", input: "", want: "/"},
		{name: "absolute path", input: "/var/log", want: "/var/log"},
		{name: "relative path becomes absolute", input: "var/log", want: "/var/log"},
		{name: "cleans duplicate separators", input: "/var//log/", want: "/var/log"},
		{name: "rejects parent traversal", input: "/var/../etc/passwd", wantErr: true},
		{name: "rejects nul", input: "/tmp/\x00bad", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := cleanRemotePath(test.input)
			if test.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("path = %q, want %q", got, test.want)
			}
		})
	}
}
