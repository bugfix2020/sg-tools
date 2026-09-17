//go:build !windows

package windows

import (
	"context"
	"time"

	"sg-tools/pkg/clicker"
)

type KeySender struct{}

func NewKeySender(time.Duration) *KeySender { return &KeySender{} }
func (*KeySender) Press(context.Context, clicker.WindowTarget, uint16) error {
	return clicker.ErrUnsupportedPlatform
}
func (*KeySender) SendKey(context.Context, clicker.WindowTarget, clicker.KeyTransition) error {
	return clicker.ErrUnsupportedPlatform
}

func ValidateWindow(clicker.WindowTarget) error { return clicker.ErrUnsupportedPlatform }

type WindowPicker struct{}

func NewWindowPicker() *WindowPicker { return &WindowPicker{} }
func (*WindowPicker) Begin(context.Context, clicker.WindowIdentity) (<-chan clicker.WindowPickEvent, error) {
	return nil, clicker.ErrUnsupportedPlatform
}
func (*WindowPicker) Cancel() error { return clicker.ErrUnsupportedPlatform }

type HotkeyManager struct{}

func NewHotkeyManager(func(string)) *HotkeyManager         { return &HotkeyManager{} }
func (*HotkeyManager) Register(clicker.HotkeyConfig) error { return clicker.ErrUnsupportedPlatform }
func (*HotkeyManager) Unregister() error                   { return clicker.ErrUnsupportedPlatform }

func PlaySound(string) error                   { return clicker.ErrUnsupportedPlatform }
func RestartAsAdministrator() error            { return clicker.ErrUnsupportedPlatform }
func ResolveVirtualKey(string) (uint16, error) { return 0, clicker.ErrUnsupportedPlatform }
