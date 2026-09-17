package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"sg-tools/pkg/clicker"
	"sg-tools/pkg/windowsync"
)

type followPickerStub struct {
	events      chan clicker.WindowPickEvent
	cancelCount int
}

func (p *followPickerStub) Begin(context.Context, clicker.WindowIdentity) (<-chan clicker.WindowPickEvent, error) {
	return p.events, nil
}

func (p *followPickerStub) Cancel() error {
	p.cancelCount++
	return nil
}

type followCaptureStub struct {
	events chan windowsync.InputEvent
	once   sync.Once
}

func (c *followCaptureStub) Start(context.Context, windowsync.CaptureSpec) (<-chan windowsync.InputEvent, error) {
	return c.events, nil
}

func (c *followCaptureStub) Stop() error {
	c.once.Do(func() { close(c.events) })
	return nil
}

type followSenderStub struct{}

func (followSenderStub) SendKey(context.Context, clicker.WindowTarget, clicker.KeyTransition) error {
	return nil
}

func TestFollowSyncStartupRestoresRulesButLeavesRuntimeWindowsUnbound(t *testing.T) {
	store := clicker.NewConfigStore(t.TempDir() + "/config.json")
	want := clicker.DefaultConfig()
	want.FollowSync = clicker.KeyRuleConfig{Include: []string{"KeyA"}, Exclude: []string{"Escape"}}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	core := windowsync.NewService(windowsync.Dependencies{})
	service := newFollowSyncServiceWithDeps(core, store, &followPickerStub{events: make(chan clicker.WindowPickEvent)})
	if err := service.startup(); err != nil {
		t.Fatal(err)
	}
	snapshot := service.GetSnapshot()
	if snapshot.Main != nil || len(snapshot.Follows) != 0 {
		t.Fatalf("expected runtime windows to be empty after startup, got main=%+v follows=%+v", snapshot.Main, snapshot.Follows)
	}
	if len(snapshot.Rules.Include) != 1 || snapshot.Rules.Include[0] != "KeyA" {
		t.Fatalf("expected persisted rules, got %+v", snapshot.Rules)
	}
}

func TestFollowSyncMainPickerRoutesCompletedWindowToMainOnly(t *testing.T) {
	store := clicker.NewConfigStore(t.TempDir() + "/config.json")
	picker := &followPickerStub{events: make(chan clicker.WindowPickEvent, 1)}
	service := newFollowSyncServiceWithDeps(windowsync.NewService(windowsync.Dependencies{}), store, picker)
	if err := service.BeginMainWindowPick(); err != nil {
		t.Fatal(err)
	}
	picker.events <- clicker.WindowPickEvent{Kind: "complete", Target: &clicker.WindowTarget{Handle: 101, Title: "Main", PID: 1}}
	waitForFollowSync(t, func() bool { return service.GetSnapshot().Main != nil })
	if got := service.GetSnapshot(); len(got.Follows) != 0 || got.Main.Title != "Main" {
		t.Fatalf("main pick routed incorrectly: %+v", got)
	}
}

func TestFollowSyncRejectsRuleMutationWhileRunning(t *testing.T) {
	store := clicker.NewConfigStore(t.TempDir() + "/config.json")
	core := windowsync.NewService(windowsync.Dependencies{
		Capture: &followCaptureStub{events: make(chan windowsync.InputEvent, 1)},
		Sender:  followSenderStub{},
	})
	service := newFollowSyncServiceWithDeps(core, store, &followPickerStub{events: make(chan clicker.WindowPickEvent)})
	if err := core.SetMainTarget(clicker.WindowTarget{Handle: 101}); err != nil {
		t.Fatal(err)
	}
	if err := core.SetFollowTargets([]clicker.WindowTarget{{Handle: 202}}); err != nil {
		t.Fatal(err)
	}
	if err := core.Start(); err != nil {
		t.Fatal(err)
	}
	if err := service.SetRules(clicker.KeyRuleConfig{Include: []string{"KeyA"}}); err == nil {
		t.Fatal("expected running service to reject rule mutation")
	}
	_ = core.Stop()
}

func TestFollowSyncCancelWindowPickDelegatesToPicker(t *testing.T) {
	picker := &followPickerStub{events: make(chan clicker.WindowPickEvent)}
	service := newFollowSyncServiceWithDeps(windowsync.NewService(windowsync.Dependencies{}), clicker.NewConfigStore(t.TempDir()+"/config.json"), picker)

	if err := service.CancelWindowPick(); err != nil {
		t.Fatal(err)
	}
	if picker.cancelCount != 1 {
		t.Fatalf("expected picker cancellation once, got %d", picker.cancelCount)
	}
}

func waitForFollowSync(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !condition() {
		t.Fatal("condition was not reached before timeout")
	}
}
