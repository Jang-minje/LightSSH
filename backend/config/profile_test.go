package config

import "testing"

func TestConnectionProfileValidate(t *testing.T) {
	tests := []struct {
		name    string
		profile ConnectionProfile
		wantErr bool
	}{
		{
			name: "valid password profile",
			profile: ConnectionProfile{
				ID:       "prod-1",
				Name:     "Production",
				Host:     "prod.example.internal",
				Port:     22,
				Username: "deploy",
				AuthType: AuthTypePassword,
			},
		},
		{
			name: "valid ip profile",
			profile: ConnectionProfile{
				ID:       "host-1",
				Name:     "Host",
				Host:     "192.168.0.10",
				Port:     2222,
				Username: "ops",
				AuthType: AuthTypeAgent,
			},
		},
		{
			name: "missing host",
			profile: ConnectionProfile{
				ID:       "bad-1",
				Name:     "Bad",
				Port:     22,
				Username: "ops",
				AuthType: AuthTypePassword,
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			profile: ConnectionProfile{
				ID:       "bad-2",
				Name:     "Bad",
				Host:     "example.internal",
				Port:     70000,
				Username: "ops",
				AuthType: AuthTypePassword,
			},
			wantErr: true,
		},
		{
			name: "missing username",
			profile: ConnectionProfile{
				ID:       "bad-3",
				Name:     "Bad",
				Host:     "example.internal",
				Port:     22,
				AuthType: AuthTypePassword,
			},
			wantErr: true,
		},
		{
			name: "private key requires key path",
			profile: ConnectionProfile{
				ID:       "bad-4",
				Name:     "Bad",
				Host:     "example.internal",
				Port:     22,
				Username: "ops",
				AuthType: AuthTypeKey,
			},
			wantErr: true,
		},
		{
			name: "unsupported auth type",
			profile: ConnectionProfile{
				ID:       "bad-5",
				Name:     "Bad",
				Host:     "example.internal",
				Port:     22,
				Username: "ops",
				AuthType: "token",
			},
			wantErr: true,
		},
		{
			name: "host with whitespace",
			profile: ConnectionProfile{
				ID:       "bad-6",
				Name:     "Bad",
				Host:     "example.internal ",
				Port:     22,
				Username: "ops",
				AuthType: AuthTypePassword,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.profile.Validate()
			if test.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateProfilesRejectsDuplicateIDs(t *testing.T) {
	profiles := []ConnectionProfile{
		{
			ID:       "same",
			Name:     "One",
			Host:     "one.example.internal",
			Port:     22,
			Username: "ops",
			AuthType: AuthTypePassword,
		},
		{
			ID:       "same",
			Name:     "Two",
			Host:     "two.example.internal",
			Port:     22,
			Username: "ops",
			AuthType: AuthTypePassword,
		},
	}

	if err := ValidateProfiles(profiles); err == nil {
		t.Fatal("expected duplicate id validation error")
	}
}
