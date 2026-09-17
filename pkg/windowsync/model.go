package windowsync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"sg-tools/pkg/clicker"
)

var (
	ErrInvalidRule         = errors.New("invalid key rule")
	ErrDuplicateRule       = errors.New("duplicate key rule")
	ErrConflictingRule     = errors.New("conflicting key rule")
	ErrSyncRunning         = errors.New("window sync is running")
	ErrNoMainWindow        = errors.New("main window is not bound")
	ErrNoFollowWindows     = errors.New("no follow windows are bound")
	ErrFollowWindowMissing = errors.New("follow window not found")
	ErrTargetInvalid       = errors.New("sync target is invalid")
	ErrCaptureUnavailable  = errors.New("input capture is unavailable")
	ErrSenderUnavailable   = errors.New("key event sender is unavailable")
)

type FilterConfig struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

func normalizeRule(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func (f FilterConfig) Validate() error {
	include := make(map[string]struct{}, len(f.Include))
	for _, code := range f.Include {
		normalized := normalizeRule(code)
		if normalized == "" {
			return fmt.Errorf("%w: include rule cannot be blank", ErrInvalidRule)
		}
		if _, exists := include[normalized]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateRule, normalized)
		}
		include[normalized] = struct{}{}
	}

	exclude := make(map[string]struct{}, len(f.Exclude))
	for _, code := range f.Exclude {
		normalized := normalizeRule(code)
		if normalized == "" {
			return fmt.Errorf("%w: exclude rule cannot be blank", ErrInvalidRule)
		}
		if _, exists := exclude[normalized]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateRule, normalized)
		}
		if _, exists := include[normalized]; exists {
			return fmt.Errorf("%w: %q", ErrConflictingRule, normalized)
		}
		exclude[normalized] = struct{}{}
	}
	return nil
}

func (f FilterConfig) Allows(code string) bool {
	normalized := normalizeRule(code)
	for _, excluded := range f.Exclude {
		if normalizeRule(excluded) == normalized {
			return false
		}
	}
	if len(f.Include) == 0 {
		return true
	}
	for _, included := range f.Include {
		if normalizeRule(included) == normalized {
			return true
		}
	}
	return false
}

type InputKind string

const (
	InputKindKeyboard InputKind = "keyboard"
)

type InputEvent struct {
	Kind         InputKind
	Code         string
	Label        string
	SourceHandle uintptr
	Transition   clicker.KeyTransition
}

type CaptureSpec struct {
	Main clicker.WindowTarget
}

type InputCapture interface {
	Start(context.Context, CaptureSpec) (<-chan InputEvent, error)
	Stop() error
}

type Dependencies struct {
	Capture        InputCapture
	Sender         clicker.KeyEventSender
	ValidateTarget func(clicker.WindowTarget) error
	OnStateChanged func(StateEvent)
}

type State string

const (
	StateIdle    State = "idle"
	StateReady   State = "ready"
	StateRunning State = "running"
	StateError   State = "error"
)

type StateEvent struct {
	State         State
	CapturedCount uint64
	LastCode      string
	Error         string
}

type Snapshot struct {
	State         State
	Main          *clicker.WindowTarget
	Follows       []clicker.WindowTarget
	Filter        FilterConfig
	CapturedCount uint64
	LastCode      string
	LastError     string
}
