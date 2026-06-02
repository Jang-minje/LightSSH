package config

import (
	"path/filepath"
	"testing"
)

func TestStoreSaveAndLoadProfiles(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	want := []ConnectionProfile{
		{
			ID:          "dev-1",
			Name:        "Development",
			Host:        "dev.example.internal",
			Port:        22,
			Username:    "developer",
			AuthType:    AuthTypeKey,
			KeyPath:     "C:/Users/user/.ssh/id_ed25519",
			Description: "Development server",
		},
	}

	if err := store.SaveProfiles(want); err != nil {
		t.Fatalf("save profiles: %v", err)
	}

	got, err := store.LoadProfiles()
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("profile count = %d, want %d", len(got), len(want))
	}
	if got[0] != want[0] {
		t.Fatalf("profile = %#v, want %#v", got[0], want[0])
	}
}

func TestStoreLoadMissingFileReturnsEmptyProfiles(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	profiles, err := store.LoadProfiles()
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("profiles = %d, want 0", len(profiles))
	}
}
