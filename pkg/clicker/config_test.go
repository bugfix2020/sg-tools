package clicker

import (
	"path/filepath"
	"testing"
)

func TestConfigStoreRoundTripsProfilesAndHotkeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewConfigStore(path)
	want := AppConfig{
		Version:  1,
		Hotkeys:  DefaultHotkeys(),
		Profiles: []PersistedProfile{{ID: "p1", Name: "Tab 1", Bindings: make([]KeyBinding, MaxBindings)}},
	}
	want.Profiles[0].Bindings[0] = KeyBinding{Code: "KeyA", Label: "A", DelayMs: 100}

	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles[0].Bindings[0] != want.Profiles[0].Bindings[0] {
		t.Fatalf("round trip binding mismatch: got %+v want %+v", got.Profiles[0].Bindings[0], want.Profiles[0].Bindings[0])
	}
}

func TestConfigStoreReturnsDefaultsForCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewConfigStore(path)
	if err := store.writeRaw([]byte("{invalid")); err != nil {
		t.Fatal(err)
	}

	got, err := store.Load()
	if err == nil {
		t.Fatal("expected corrupt config error")
	}
	if got.Version != 1 || got.Hotkeys != DefaultHotkeys() {
		t.Fatalf("expected default config, got %+v", got)
	}
}
