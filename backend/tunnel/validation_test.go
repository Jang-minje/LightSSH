package tunnel

import "testing"

func TestLocalConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  LocalConfig
		wantErr bool
	}{
		{
			name: "valid",
			config: LocalConfig{
				ID:               "db",
				Name:             "DB",
				LocalBindAddress: "127.0.0.1",
				LocalPort:        15432,
				TargetHost:       "db.internal",
				TargetPort:       5432,
			},
		},
		{
			name: "allows system assigned local port for tests",
			config: LocalConfig{
				ID:               "db",
				Name:             "DB",
				LocalBindAddress: "127.0.0.1",
				LocalPort:        0,
				TargetHost:       "db.internal",
				TargetPort:       5432,
			},
		},
		{
			name: "missing bind address",
			config: LocalConfig{
				ID:         "bad",
				Name:       "Bad",
				LocalPort:  15432,
				TargetHost: "db.internal",
				TargetPort: 5432,
			},
			wantErr: true,
		},
		{
			name: "invalid local port",
			config: LocalConfig{
				ID:               "bad",
				Name:             "Bad",
				LocalBindAddress: "127.0.0.1",
				LocalPort:        70000,
				TargetHost:       "db.internal",
				TargetPort:       5432,
			},
			wantErr: true,
		},
		{
			name: "target host whitespace",
			config: LocalConfig{
				ID:               "bad",
				Name:             "Bad",
				LocalBindAddress: "127.0.0.1",
				LocalPort:        15432,
				TargetHost:       "db internal",
				TargetPort:       5432,
			},
			wantErr: true,
		},
		{
			name: "invalid target port",
			config: LocalConfig{
				ID:               "bad",
				Name:             "Bad",
				LocalBindAddress: "127.0.0.1",
				LocalPort:        15432,
				TargetHost:       "db.internal",
				TargetPort:       0,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if test.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestReverseConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  ReverseConfig
		wantErr bool
	}{
		{
			name: "valid",
			config: ReverseConfig{
				ID:                "rev-db",
				Name:              "Reverse DB",
				RemoteBindAddress: "127.0.0.1",
				RemotePort:        15432,
				LocalTargetHost:   "127.0.0.1",
				LocalTargetPort:   5432,
			},
		},
		{
			name: "invalid remote bind",
			config: ReverseConfig{
				ID:                "bad",
				Name:              "Bad",
				RemoteBindAddress: "bad address",
				RemotePort:        15432,
				LocalTargetHost:   "127.0.0.1",
				LocalTargetPort:   5432,
			},
			wantErr: true,
		},
		{
			name: "remote port cannot be zero",
			config: ReverseConfig{
				ID:                "bad",
				Name:              "Bad",
				RemoteBindAddress: "127.0.0.1",
				RemotePort:        0,
				LocalTargetHost:   "127.0.0.1",
				LocalTargetPort:   5432,
			},
			wantErr: true,
		},
		{
			name: "local target required",
			config: ReverseConfig{
				ID:                "bad",
				Name:              "Bad",
				RemoteBindAddress: "127.0.0.1",
				RemotePort:        15432,
				LocalTargetPort:   5432,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if test.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
