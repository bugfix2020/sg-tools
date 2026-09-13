//go:build windows

package windows

import "testing"

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
