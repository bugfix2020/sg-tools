package windowsync

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"sg-tools/pkg/clicker"
)

type captureStub struct {
	mu       sync.Mutex
	starts   []CaptureSpec
	stopCall int
	events   chan InputEvent
	err      error
	stopOnce sync.Once
}

func (c *captureStub) Start(_ context.Context, spec CaptureSpec) (<-chan InputEvent, error) {
	c.mu.Lock()
	c.starts = append(c.starts, spec)
	c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return c.events, nil
}

func (c *captureStub) Stop() error {
	c.mu.Lock()
	c.stopCall++
	c.mu.Unlock()
	c.stopOnce.Do(func() { close(c.events) })
	return nil
}

func (c *captureStub) stopCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopCall
}

type transitionRecord struct {
	target     clicker.WindowTarget
	transition clicker.KeyTransition
}

type senderStub struct {
	mu      sync.Mutex
	records []transitionRecord
	err     error
}

func (s *senderStub) SendKey(_ context.Context, target clicker.WindowTarget, transition clicker.KeyTransition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.records = append(s.records, transitionRecord{target: target, transition: transition})
	return nil
}

func (s *senderStub) snapshot() []transitionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]transitionRecord(nil), s.records...)
}

func syncTarget(handle uintptr, title string) clicker.WindowTarget {
	return clicker.WindowTarget{Handle: handle, Title: title, PID: uint32(handle)}
}

func newRunningService(capture *captureStub, sender *senderStub) (*Service, clicker.WindowTarget, []clicker.WindowTarget) {
	main := syncTarget(100, "Main")
	follows := []clicker.WindowTarget{syncTarget(200, "Follow 1"), syncTarget(300, "Follow 2")}
	service := NewService(Dependencies{
		Capture: capture,
		Sender:  sender,
		ValidateTarget: func(target clicker.WindowTarget) error {
			if target.Handle == 0 {
				return clicker.ErrWindowNotBound
			}
			return nil
		},
	})
	if err := service.SetMainTarget(main); err != nil {
		panic(err)
	}
	if err := service.SetFollowTargets(follows); err != nil {
		panic(err)
	}
	return service, main, follows
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !condition() {
		t.Fatal("condition was not reached before timeout")
	}
}

func TestServiceBroadcastsMainKeyboardEventToEveryFollowInOrder(t *testing.T) {
	capture := &captureStub{events: make(chan InputEvent, 4)}
	sender := &senderStub{}
	service, main, follows := newRunningService(capture, sender)

	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	capture.events <- InputEvent{
		Kind:         InputKindKeyboard,
		Code:         "KeyA",
		SourceHandle: main.Handle,
		Transition:   clicker.KeyTransition{VirtualKey: 'A', ScanCode: 30, Down: true, System: true},
	}
	waitFor(t, func() bool { return len(sender.snapshot()) == 2 })

	records := sender.snapshot()
	for index, follow := range follows {
		if records[index].target.Handle != follow.Handle {
			t.Fatalf("record %d target = %d; want %d", index, records[index].target.Handle, follow.Handle)
		}
		if records[index].transition.VirtualKey != 'A' || !records[index].transition.Down || !records[index].transition.System {
			t.Fatalf("record %d transition = %+v; want key-down A", index, records[index].transition)
		}
	}
	if got := service.Snapshot().CapturedCount; got != 1 {
		t.Fatalf("captured count = %d; want 1", got)
	}
	_ = service.Stop()
}

func TestServiceFiltersKeyboardEventBeforeBroadcast(t *testing.T) {
	capture := &captureStub{events: make(chan InputEvent, 4)}
	sender := &senderStub{}
	service, main, _ := newRunningService(capture, sender)
	if err := service.Configure(FilterConfig{Include: []string{"KeyA"}}); err != nil {
		t.Fatal(err)
	}
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	capture.events <- InputEvent{Kind: InputKindKeyboard, Code: "KeyB", SourceHandle: main.Handle, Transition: clicker.KeyTransition{VirtualKey: 'B', Down: true}}
	time.Sleep(20 * time.Millisecond)
	if got := len(sender.snapshot()); got != 0 {
		t.Fatalf("filtered event produced %d sends; want 0", got)
	}
	_ = service.Stop()
}

func TestServiceRejectsInvalidTargetOnStart(t *testing.T) {
	capture := &captureStub{events: make(chan InputEvent, 1)}
	sender := &senderStub{}
	service := NewService(Dependencies{
		Capture: capture,
		Sender:  sender,
		ValidateTarget: func(target clicker.WindowTarget) error {
			if target.Title == "Closed" {
				return errors.New("target window is no longer valid")
			}
			return nil
		},
	})
	if err := service.SetMainTarget(syncTarget(100, "Closed")); err != nil {
		t.Fatal(err)
	}
	if err := service.SetFollowTargets([]clicker.WindowTarget{syncTarget(200, "Follow")}); err != nil {
		t.Fatal(err)
	}
	if err := service.Start(); !errors.Is(err, ErrTargetInvalid) {
		t.Fatalf("Start() error = %v; want ErrTargetInvalid", err)
	}
	if got := service.Snapshot().State; got == StateRunning {
		t.Fatalf("service state = %s after invalid start", got)
	}
}

func TestServiceStopCleansUpCaptureAndReturnsReady(t *testing.T) {
	capture := &captureStub{events: make(chan InputEvent, 1)}
	service, _, _ := newRunningService(capture, &senderStub{})
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if got := service.Snapshot().State; got != StateReady {
		t.Fatalf("state = %s; want ready", got)
	}
	if got := capture.stopCount(); got != 1 {
		t.Fatalf("capture stop calls = %d; want 1", got)
	}
}

func TestServiceSenderFailureStopsAndReportsInvalidTarget(t *testing.T) {
	capture := &captureStub{events: make(chan InputEvent, 1)}
	sender := &senderStub{err: errors.New("target window is no longer valid")}
	service, main, _ := newRunningService(capture, sender)
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	capture.events <- InputEvent{Kind: InputKindKeyboard, Code: "KeyA", SourceHandle: main.Handle, Transition: clicker.KeyTransition{VirtualKey: 'A', Down: true}}
	waitFor(t, func() bool { return service.Snapshot().State == StateError })
	if got := service.Snapshot().LastError; got == "" {
		t.Fatal("expected sender failure to be reported")
	}
	if got := capture.stopCount(); got != 1 {
		t.Fatalf("capture stop calls = %d; want 1", got)
	}
}

func TestServiceRejectsRuntimeMutationWhileRunning(t *testing.T) {
	capture := &captureStub{events: make(chan InputEvent, 1)}
	service, _, follows := newRunningService(capture, &senderStub{})
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	if err := service.Configure(FilterConfig{Exclude: []string{"KeyA"}}); !errors.Is(err, ErrSyncRunning) {
		t.Fatalf("Configure() error = %v; want ErrSyncRunning", err)
	}
	if err := service.SetMainTarget(syncTarget(101, "New Main")); !errors.Is(err, ErrSyncRunning) {
		t.Fatalf("SetMainTarget() error = %v; want ErrSyncRunning", err)
	}
	if err := service.SetFollowTargets(follows[:1]); !errors.Is(err, ErrSyncRunning) {
		t.Fatalf("SetFollowTargets() error = %v; want ErrSyncRunning", err)
	}
	if err := service.AddFollowTarget(syncTarget(400, "New Follow")); !errors.Is(err, ErrSyncRunning) {
		t.Fatalf("AddFollowTarget() error = %v; want ErrSyncRunning", err)
	}
	if err := service.RemoveFollowTarget(0); !errors.Is(err, ErrSyncRunning) {
		t.Fatalf("RemoveFollowTarget() error = %v; want ErrSyncRunning", err)
	}
	_ = service.Stop()
}

func TestServiceRejectsMainTargetAlreadyBoundAsFollow(t *testing.T) {
	service := NewService(Dependencies{})
	if err := service.SetFollowTargets([]clicker.WindowTarget{syncTarget(200, "Follow")}); err != nil {
		t.Fatal(err)
	}
	if err := service.SetMainTarget(syncTarget(200, "Main")); !errors.Is(err, ErrTargetInvalid) {
		t.Fatalf("SetMainTarget() error = %v; want ErrTargetInvalid", err)
	}
}
