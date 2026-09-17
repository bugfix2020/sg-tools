//go:build windows

package windows

import (
	"testing"

	"sg-tools/pkg/clicker"
)

func TestResolveVirtualKeySupportsBrowserKeyboardCodes(t *testing.T) {
	tests := []struct {
		code string
		want uint16
	}{
		{"KeyA", 'A'},
		{"Digit7", '7'},
		{"PageUp", 0x21},
		{"PageDown", 0x22},
		{"F12", 0x7B},
		{"Space", 0x20},
	}
	for _, test := range tests {
		got, err := ResolveVirtualKey(test.code)
		if err != nil || got != test.want {
			t.Fatalf("ResolveVirtualKey(%q) = %d, %v; want %d", test.code, got, err, test.want)
		}
	}
}

func TestKeyboardCodeNormalizesCommonLowLevelHookKeys(t *testing.T) {
	tests := []struct {
		virtualKey uint32
		want       string
	}{
		{virtualKey: 'A', want: "KeyA"},
		{virtualKey: '7', want: "Digit7"},
		{virtualKey: 0x20, want: "Space"},
		{virtualKey: 0x70, want: "F1"},
		{virtualKey: 0x21, want: "PageUp"},
	}
	for _, test := range tests {
		if got := keyboardCode(test.virtualKey, 0, 0); got != test.want {
			t.Fatalf("keyboardCode(%#x) = %q; want %q", test.virtualKey, got, test.want)
		}
	}
}

func TestWindowPickTrackerClearsCandidateWhenCursorLeavesWindow(t *testing.T) {
	tracker := newWindowPickTracker()
	target := &clicker.WindowTarget{Handle: 101, Title: "Demo"}

	if !tracker.Update(target) {
		t.Fatal("expected the first target to be a candidate change")
	}
	if tracker.Current() != target {
		t.Fatalf("expected current candidate %p, got %p", target, tracker.Current())
	}

	if !tracker.Update(nil) {
		t.Fatal("expected leaving the target to be a candidate change")
	}
	if tracker.Current() != nil {
		t.Fatalf("expected no current candidate after leaving the window, got %+v", tracker.Current())
	}
}

func TestWindowPickerCancellationBelongsOnlyToCurrentGeneration(t *testing.T) {
	if shouldEmitWindowPickCancellation(2, 1) {
		t.Fatal("expected a stale picker cancellation to be ignored")
	}
	if !shouldEmitWindowPickCancellation(2, 2) {
		t.Fatal("expected the current picker cancellation to be emitted")
	}
}
