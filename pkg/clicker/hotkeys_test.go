package clicker

import (
	"errors"
	"testing"
)

func TestDefaultHotkeysUsePageNavigation(t *testing.T) {
	got := DefaultHotkeys()
	if got.ActiveStart != "pageup" || got.ActiveStop != "pagedown" || got.GlobalStart != "ctrl+pageup" || got.GlobalStop != "ctrl+pagedown" {
		t.Fatalf("unexpected defaults: %+v", got)
	}
}

func TestValidateHotkeysRejectsDuplicatesAndModifierOnly(t *testing.T) {
	valid := DefaultHotkeys()
	if err := ValidateHotkeys(valid); err != nil {
		t.Fatalf("default hotkeys should be valid: %v", err)
	}

	duplicate := valid
	duplicate.GlobalStart = duplicate.ActiveStart
	if !errors.Is(ValidateHotkeys(duplicate), ErrDuplicateHotkey) {
		t.Fatal("expected duplicate hotkey error")
	}

	modifierOnly := valid
	modifierOnly.ActiveStart = "ctrl"
	if !errors.Is(ValidateHotkeys(modifierOnly), ErrInvalidHotkey) {
		t.Fatal("expected modifier-only hotkey error")
	}
}
