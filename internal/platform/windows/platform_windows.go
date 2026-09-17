//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"sg-tools/pkg/clicker"
)

const (
	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	wmHotKey     = 0x0312
	pmRemove     = 0x0001
	gaRoot       = 2
	vkLButton    = 0x01
	modAlt       = 0x0001
	modControl   = 0x0002
	modShift     = 0x0004
	modWin       = 0x0008
	modNoRepeat  = 0x4000
	sndFilename  = 0x00020000
	sndAsync     = 0x0001

	activeStartID = 1
	activeStopID  = 2
	globalStartID = 3
	globalStopID  = 4
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procPostMsg                  = user32.NewProc("PostMessageW")
	procMapVK                    = user32.NewProc("MapVirtualKeyW")
	procIsWindow                 = user32.NewProc("IsWindow")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procWindowFromPoint          = user32.NewProc("WindowFromPoint")
	procGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")
	procGetAncestor              = user32.NewProc("GetAncestor")
	procGetWindowTextLength      = user32.NewProc("GetWindowTextLengthW")
	procGetWindowText            = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procRegisterHotKey           = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey         = user32.NewProc("UnregisterHotKey")
	procPeekMessage              = user32.NewProc("PeekMessageW")
	shell32                      = windows.NewLazySystemDLL("shell32.dll")
	procShellExecute             = shell32.NewProc("ShellExecuteW")
	winmm                        = windows.NewLazySystemDLL("winmm.dll")
	procPlaySound                = winmm.NewProc("PlaySoundW")
)

type point struct{ x, y int32 }
type message struct {
	hwnd   uintptr
	msg    uint32
	wParam uintptr
	lParam uintptr
	time   uint32
	pt     point
}

type KeySender struct {
	hold time.Duration
}

func NewKeySender(hold time.Duration) *KeySender {
	if hold <= 0 {
		hold = 5 * time.Millisecond
	}
	return &KeySender{hold: hold}
}

func (s *KeySender) Press(ctx context.Context, target clicker.WindowTarget, virtualKey uint16) error {
	if err := s.SendKey(ctx, target, clicker.KeyTransition{VirtualKey: virtualKey, Down: true}); err != nil {
		return err
	}
	if err := waitContext(ctx, s.hold); err != nil {
		return err
	}
	return s.SendKey(ctx, target, clicker.KeyTransition{VirtualKey: virtualKey, Down: false})
}

func (s *KeySender) SendKey(_ context.Context, target clicker.WindowTarget, transition clicker.KeyTransition) error {
	if target.Handle == 0 {
		return clicker.ErrWindowNotBound
	}
	if ok, _, _ := procIsWindow.Call(target.Handle); ok == 0 {
		return fmt.Errorf("target window is no longer valid")
	}
	scanCode := uintptr(transition.ScanCode)
	if scanCode == 0 {
		scanCode, _, _ = procMapVK.Call(uintptr(transition.VirtualKey), 0)
	}
	lParam := uintptr(1) | (scanCode << 16)
	if transition.Extended {
		lParam |= 1 << 24
	}
	messageID := wmKeyDown
	operation := "WM_KEYDOWN"
	if !transition.Down {
		messageID = wmKeyUp
		lParam |= (1 << 30) | (1 << 31)
		operation = "WM_KEYUP"
	}
	if transition.System {
		if transition.Down {
			messageID = wmSysKeyDown
			operation = "WM_SYSKEYDOWN"
		} else {
			messageID = wmSysKeyUp
			operation = "WM_SYSKEYUP"
		}
	}
	if result, _, callErr := procPostMsg.Call(target.Handle, uintptr(messageID), uintptr(transition.VirtualKey), lParam); result == 0 {
		return win32CallError(operation, callErr)
	}
	return nil
}

func ValidateWindow(target clicker.WindowTarget) error {
	if target.Handle == 0 {
		return clicker.ErrWindowNotBound
	}
	if ok, _, _ := procIsWindow.Call(target.Handle); ok == 0 {
		return fmt.Errorf("target window is no longer valid")
	}
	return nil
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func win32CallError(operation string, callErr error) error {
	if callErr == nil || errors.Is(callErr, syscall.Errno(0)) {
		callErr = windows.GetLastError()
	}
	return fmt.Errorf("%s failed: %w", operation, callErr)
}

func ResolveVirtualKey(code string) (uint16, error) {
	normalized := strings.ToLower(strings.TrimSpace(code))
	if len(normalized) == 4 && strings.HasPrefix(normalized, "key") {
		value := normalized[3]
		if value >= 'a' && value <= 'z' {
			return uint16(strings.ToUpper(string(value))[0]), nil
		}
	}
	if len(normalized) == 6 && strings.HasPrefix(normalized, "digit") {
		value := normalized[5]
		if value >= '0' && value <= '9' {
			return uint16(value), nil
		}
	}
	if strings.HasPrefix(normalized, "f") {
		var number int
		if _, err := fmt.Sscanf(normalized, "f%d", &number); err == nil && number >= 1 && number <= 24 {
			return uint16(0x70 + number - 1), nil
		}
	}
	keys := map[string]uint16{
		"space": 0x20, "enter": 0x0D, "escape": 0x1B, "esc": 0x1B, "tab": 0x09,
		"backspace": 0x08, "delete": 0x2E, "del": 0x2E, "insert": 0x2D,
		"home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22,
		"arrowup": 0x26, "arrowdown": 0x28, "arrowleft": 0x25, "arrowright": 0x27,
		"minus": 0xBD, "equal": 0xBB, "comma": 0xBC, "period": 0xBE,
		"slash": 0xBF, "semicolon": 0xBA, "quote": 0xDE, "bracketleft": 0xDB,
		"bracketright": 0xDD, "backslash": 0xDC, "backquote": 0xC0,
	}
	if key, ok := keys[normalized]; ok {
		return key, nil
	}
	return 0, fmt.Errorf("unsupported keyboard code %q", code)
}

type WindowPicker struct {
	mu         sync.Mutex
	cancel     context.CancelFunc
	generation uint64
}

func NewWindowPicker() *WindowPicker { return &WindowPicker{} }

func (p *WindowPicker) Begin(parent context.Context, owner clicker.WindowIdentity) (<-chan clicker.WindowPickEvent, error) {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.generation++
	generation := p.generation
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	p.mu.Unlock()

	events := make(chan clicker.WindowPickEvent, 8)
	go func() {
		defer close(events)
		defer func() {
			p.mu.Lock()
			if p.generation == generation {
				p.cancel = nil
			}
			p.mu.Unlock()
		}()
		tracker := newWindowPickTracker()
		wasDown := false
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				p.mu.Lock()
				currentGeneration := p.generation
				p.mu.Unlock()
				if shouldEmitWindowPickCancellation(currentGeneration, generation) {
					events <- clicker.WindowPickEvent{Kind: "cancelled"}
				}
				return
			case <-ticker.C:
				down := mouseLeftDown()
				if down {
					wasDown = true
					target := targetAtCursor(owner)
					if tracker.Update(target) {
						events <- clicker.WindowPickEvent{Kind: "preview", Target: target}
					}
					continue
				}
				if wasDown {
					if tracker.Current() != nil {
						events <- clicker.WindowPickEvent{Kind: "complete", Target: tracker.Current()}
					} else {
						events <- clicker.WindowPickEvent{Kind: "cancelled"}
					}
					return
				}
			}
		}
	}()
	return events, nil
}

func (p *WindowPicker) Cancel() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	return nil
}

func shouldEmitWindowPickCancellation(currentGeneration, eventGeneration uint64) bool {
	return currentGeneration == eventGeneration
}

type windowPickTracker struct {
	lastHandle uintptr
	current    *clicker.WindowTarget
}

func newWindowPickTracker() *windowPickTracker { return &windowPickTracker{} }

func (t *windowPickTracker) Update(target *clicker.WindowTarget) bool {
	handle := uintptr(0)
	if target != nil {
		handle = target.Handle
	}
	if handle == t.lastHandle {
		return false
	}
	t.lastHandle = handle
	t.current = target
	return true
}

func (t *windowPickTracker) Current() *clicker.WindowTarget { return t.current }

func mouseLeftDown() bool {
	value, _, _ := procGetAsyncKeyState.Call(vkLButton)
	return value&0x8000 != 0
}

func targetAtCursor(owner clicker.WindowIdentity) *clicker.WindowTarget {
	var cursor point
	if result, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); result == 0 {
		return nil
	}
	hwnd, _, _ := procWindowFromPoint.Call(uintptr(uint64(uint32(cursor.x)) | uint64(uint32(cursor.y))<<32))
	if hwnd == 0 || hwnd == owner.Handle || isCurrentProcessWindow(hwnd) {
		return nil
	}
	root, _, _ := procGetAncestor.Call(hwnd, gaRoot)
	if root == 0 {
		root = hwnd
	}
	title := windowText(root)
	var pid uint32
	procGetWindowThreadProcessID.Call(root, uintptr(unsafe.Pointer(&pid)))
	return &clicker.WindowTarget{Handle: hwnd, Title: title, PID: pid}
}

func isCurrentProcessWindow(hwnd uintptr) bool {
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid == uint32(os.Getpid())
}

func windowText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLength.Call(hwnd)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, length+1)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), length+1)
	return windows.UTF16ToString(buffer)
}

type HotkeyManager struct {
	mu     sync.Mutex
	stop   chan struct{}
	done   chan struct{}
	action func(string)
}

func NewHotkeyManager(action func(string)) *HotkeyManager {
	return &HotkeyManager{action: action}
}

func (m *HotkeyManager) Register(config clicker.HotkeyConfig) error {
	if err := clicker.ValidateHotkeys(config); err != nil {
		return err
	}
	_ = m.Unregister()
	stop := make(chan struct{})
	done := make(chan struct{})
	ready := make(chan error, 1)
	m.mu.Lock()
	m.stop, m.done = stop, done
	m.mu.Unlock()
	go m.run(config, stop, done, ready)
	return <-ready
}

func (m *HotkeyManager) run(config clicker.HotkeyConfig, stop <-chan struct{}, done chan<- struct{}, ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)
	entries := []struct {
		id     int
		hotkey string
		action string
	}{
		{activeStartID, config.ActiveStart, "active-start"},
		{activeStopID, config.ActiveStop, "active-stop"},
		{globalStartID, config.GlobalStart, "global-start"},
		{globalStopID, config.GlobalStop, "global-stop"},
	}
	registered := []int{}
	for _, entry := range entries {
		modifier, key, err := parseHotkey(entry.hotkey)
		if err != nil {
			ready <- err
			return
		}
		result, _, callErr := procRegisterHotKey.Call(0, uintptr(entry.id), uintptr(modifier), uintptr(key))
		if result == 0 {
			for _, id := range registered {
				procUnregisterHotKey.Call(0, uintptr(id))
			}
			ready <- win32CallError("RegisterHotKey", callErr)
			return
		}
		registered = append(registered, entry.id)
	}
	ready <- nil
	for {
		select {
		case <-stop:
			for _, id := range registered {
				procUnregisterHotKey.Call(0, uintptr(id))
			}
			return
		default:
		}
		var msg message
		result, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, pmRemove)
		if result != 0 && msg.msg == wmHotKey {
			action := map[int]string{activeStartID: "active-start", activeStopID: "active-stop", globalStartID: "global-start", globalStopID: "global-stop"}[int(msg.wParam)]
			if action != "" && m.action != nil {
				m.action(action)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (m *HotkeyManager) Unregister() error {
	m.mu.Lock()
	stop, done := m.stop, m.done
	m.stop, m.done = nil, nil
	m.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
	return nil
}

func parseHotkey(value string) (uint32, uint16, error) {
	parts := strings.Split(clicker.NormalizeHotkey(value), "+")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return 0, 0, clicker.ErrInvalidHotkey
	}
	var modifiers uint32
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "ctrl":
			modifiers |= modControl
		case "alt":
			modifiers |= modAlt
		case "shift":
			modifiers |= modShift
		case "win":
			modifiers |= modWin
		}
	}
	key, err := ResolveVirtualKey(parts[len(parts)-1])
	if err != nil {
		return 0, 0, err
	}
	return modifiers | modNoRepeat, key, nil
}

func PlaySound(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	result, _, callErr := procPlaySound.Call(uintptr(unsafe.Pointer(wide)), 0, sndFilename|sndAsync)
	if result == 0 {
		return win32CallError("PlaySoundW", callErr)
	}
	return nil
}

func RestartAsAdministrator() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(executable)
	workingDir, _ := windows.UTF16PtrFromString(filepath.Dir(executable))
	result, _, callErr := procShellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, uintptr(unsafe.Pointer(workingDir)), 1)
	if result <= 32 {
		return win32CallError("ShellExecuteW", callErr)
	}
	return nil
}
