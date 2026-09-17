//go:build !windows

package windows

import (
	"context"

	"sg-tools/pkg/clicker"
	"sg-tools/pkg/windowsync"
)

type InputCapture struct{}

func NewInputCapture() *InputCapture { return &InputCapture{} }

func (*InputCapture) Start(context.Context, windowsync.CaptureSpec) (<-chan windowsync.InputEvent, error) {
	return nil, clicker.ErrUnsupportedPlatform
}

func (*InputCapture) Stop() error { return clicker.ErrUnsupportedPlatform }
