package clicker

import (
	"fmt"
	"strings"
)

type HotkeyConfig struct {
	ActiveStart string `json:"activeStart"`
	ActiveStop  string `json:"activeStop"`
	GlobalStart string `json:"globalStart"`
	GlobalStop  string `json:"globalStop"`
}

func DefaultHotkeys() HotkeyConfig {
	return HotkeyConfig{
		ActiveStart: "pageup",
		ActiveStop:  "pagedown",
		GlobalStart: "ctrl+pageup",
		GlobalStop:  "ctrl+pagedown",
	}
}

func NormalizeHotkey(value string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "+")
	modifiers := map[string]bool{}
	key := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch part {
		case "control":
			part = "ctrl"
		case "pgup":
			part = "pageup"
		case "pgdn":
			part = "pagedown"
		}
		if part == "ctrl" || part == "alt" || part == "shift" || part == "win" {
			modifiers[part] = true
			continue
		}
		key = part
	}
	ordered := []string{}
	for _, modifier := range []string{"ctrl", "alt", "shift", "win"} {
		if modifiers[modifier] {
			ordered = append(ordered, modifier)
		}
	}
	if key != "" {
		ordered = append(ordered, key)
	}
	return strings.Join(ordered, "+")
}

func ValidateHotkeys(config HotkeyConfig) error {
	values := []string{config.ActiveStart, config.ActiveStop, config.GlobalStart, config.GlobalStop}
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := NormalizeHotkey(value)
		parts := strings.Split(normalized, "+")
		if normalized == "" || parts[len(parts)-1] == "ctrl" || parts[len(parts)-1] == "alt" || parts[len(parts)-1] == "shift" || parts[len(parts)-1] == "win" {
			return fmt.Errorf("%w: %q", ErrInvalidHotkey, value)
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateHotkey, normalized)
		}
		seen[normalized] = struct{}{}
	}
	return nil
}
