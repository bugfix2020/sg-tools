//go:build windows

package windows

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"sg-tools/pkg/clicker"
	"sg-tools/pkg/windowsync"
)

const (
	whKeyboardLL = 13
	mapVKToVSC   = 0
)

var (
	procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHook   = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procRtlMoveMemory       = kernel32.NewProc("RtlMoveMemory")
)

type keyboardHookStruct struct {
	virtualKeyCode uint32
	scanCode       uint32
	flags          uint32
	time           uint32
	extraInfo      uintptr
}

type InputCapture struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewInputCapture() *InputCapture { return &InputCapture{} }

func (c *InputCapture) Start(parent context.Context, spec windowsync.CaptureSpec) (<-chan windowsync.InputEvent, error) {
	_ = c.Stop()
	ctx, cancel := context.WithCancel(parent)
	events := make(chan windowsync.InputEvent, 256)
	ready := make(chan error, 1)
	done := make(chan struct{})
	c.mu.Lock()
	c.cancel = cancel
	c.done = done
	c.mu.Unlock()
	go c.run(ctx, spec, events, ready, done)
	if err := <-ready; err != nil {
		cancel()
		<-done
		return nil, err
	}
	return events, nil
}

func (c *InputCapture) run(ctx context.Context, spec windowsync.CaptureSpec, events chan windowsync.InputEvent, ready chan<- error, done chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)

	mainRoot := rootHandle(spec.Main.Handle)
	callback := syscall.NewCallback(func(rawCode, messageID, rawData uintptr) uintptr {
		code := int32(rawCode)
		if code >= 0 && (uint32(messageID) == wmKeyDown || uint32(messageID) == wmKeyUp || uint32(messageID) == wmSysKeyDown || uint32(messageID) == wmSysKeyUp) {
			data, validData := readKeyboardHookStruct(rawData)
			foreground, _, _ := procGetForegroundWindow.Call()
			if validData && captureWindow(foreground, mainRoot) && !isCurrentProcessWindow(foreground) {
				event := windowsync.InputEvent{
					Kind:         windowsync.InputKindKeyboard,
					Code:         keyboardCode(data.virtualKeyCode, data.scanCode, data.flags),
					Label:        keyboardCode(data.virtualKeyCode, data.scanCode, data.flags),
					SourceHandle: spec.Main.Handle,
					Transition: clicker.KeyTransition{
						VirtualKey: uint16(data.virtualKeyCode),
						ScanCode:   uint16(data.scanCode),
						Extended:   data.flags&0x01 != 0,
						Down:       uint32(messageID) == wmKeyDown || uint32(messageID) == wmSysKeyDown,
						System:     uint32(messageID) == wmSysKeyDown || uint32(messageID) == wmSysKeyUp,
					},
				}
				select {
				case events <- event:
				default:
				}
			}
		}
		result, _, _ := procCallNextHookEx.Call(0, rawCode, messageID, rawData)
		return result
	})
	hook, _, callErr := procSetWindowsHookEx.Call(whKeyboardLL, callback, 0, 0)
	if hook == 0 {
		ready <- win32CallError("SetWindowsHookExW", callErr)
		close(events)
		return
	}
	ready <- nil
	defer procUnhookWindowsHook.Call(hook)
	defer close(events)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg message
			procPeekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, pmRemove)
			time.Sleep(time.Millisecond)
		}
	}
}

func readKeyboardHookStruct(rawData uintptr) (keyboardHookStruct, bool) {
	if rawData == 0 {
		return keyboardHookStruct{}, false
	}
	var data keyboardHookStruct
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&data)), rawData, unsafe.Sizeof(data))
	return data, true
}

func (c *InputCapture) Stop() error {
	c.mu.Lock()
	cancel, done := c.cancel, c.done
	c.cancel, c.done = nil, nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	return nil
}

func captureWindow(hwnd, mainRoot uintptr) bool {
	if hwnd == 0 || mainRoot == 0 {
		return false
	}
	return hwnd == mainRoot || rootHandle(hwnd) == mainRoot
}

func rootHandle(hwnd uintptr) uintptr {
	if hwnd == 0 {
		return 0
	}
	root, _, _ := procGetAncestor.Call(hwnd, gaRoot)
	if root == 0 {
		return hwnd
	}
	return root
}

func keyboardCode(virtualKey, scanCode, flags uint32) string {
	if virtualKey >= 'A' && virtualKey <= 'Z' {
		return fmt.Sprintf("Key%c", virtualKey)
	}
	if virtualKey >= '0' && virtualKey <= '9' {
		return fmt.Sprintf("Digit%c", virtualKey)
	}
	if virtualKey >= 0x70 && virtualKey <= 0x87 {
		return fmt.Sprintf("F%d", virtualKey-0x6f)
	}
	if code, ok := map[uint32]string{
		0x08: "Backspace", 0x09: "Tab", 0x0D: "Enter", 0x1B: "Escape", 0x20: "Space",
		0x21: "PageUp", 0x22: "PageDown", 0x23: "End", 0x24: "Home", 0x25: "ArrowLeft",
		0x26: "ArrowUp", 0x27: "ArrowRight", 0x28: "ArrowDown", 0x2D: "Insert", 0x2E: "Delete",
		0xBA: "Semicolon", 0xBB: "Equal", 0xBC: "Comma", 0xBD: "Minus", 0xBE: "Period",
		0xBF: "Slash", 0xC0: "Backquote", 0xDB: "BracketLeft", 0xDC: "Backslash",
		0xDD: "BracketRight", 0xDE: "Quote", 0x10: "Shift", 0x11: "Control", 0x12: "Alt",
	}[virtualKey]; ok {
		return code
	}
	if scanCode != 0 {
		mapped, _, _ := procMapVK.Call(uintptr(virtualKey), mapVKToVSC)
		if mapped != 0 && flags&0x01 != 0 {
			return fmt.Sprintf("VK_%02X_EXT", virtualKey)
		}
	}
	return fmt.Sprintf("VK_%02X", virtualKey)
}

var _ windowsync.InputCapture = (*InputCapture)(nil)
var _ clicker.KeyEventSender = (*KeySender)(nil)
