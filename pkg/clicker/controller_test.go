package clicker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeSender struct {
	presses []PressRecord
	err     error
}

func fakeResolver(string) (uint16, error) { return 'A', nil }

func (f *fakeSender) Press(_ context.Context, target WindowTarget, virtualKey uint16) error {
	f.presses = append(f.presses, PressRecord{Target: target, VirtualKey: virtualKey})
	return f.err
}

func TestProfileValidationRequiresTargetAndKey(t *testing.T) {
	profile := NewProfile("p1", "Tab 1")

	if err := profile.ValidateForStart(); !errors.Is(err, ErrWindowNotBound) {
		t.Fatalf("expected ErrWindowNotBound, got %v", err)
	}

	profile.Target = &WindowTarget{Handle: 100, Title: "Notepad"}
	if err := profile.ValidateForStart(); !errors.Is(err, ErrNoEnabledKeys) {
		t.Fatalf("expected ErrNoEnabledKeys, got %v", err)
	}

	profile.Bindings[0] = KeyBinding{Code: "KeyA", Label: "A", DelayMs: 100}
	if err := profile.ValidateForStart(); err != nil {
		t.Fatalf("expected profile to be startable, got %v", err)
	}
}

func TestControllerStartStopChangesProfileState(t *testing.T) {
	sender := &fakeSender{}
	controller := NewController(Dependencies{Sender: sender, ResolveVirtualKey: fakeResolver, HoldDuration: time.Millisecond})
	profile, err := controller.CreateProfile()
	if err != nil {
		t.Fatal(err)
	}
	profile.Target = &WindowTarget{Handle: 100, Title: "Notepad"}
	profile.Bindings[0] = KeyBinding{Code: "KeyA", Label: "A", DelayMs: 1}

	if err := controller.StartProfile(profile.ID); err != nil {
		t.Fatal(err)
	}
	if got := controller.Profile(profile.ID).State; got != ProfileRunning {
		t.Fatalf("expected running state, got %s", got)
	}

	controller.StopProfile(profile.ID)
	if got := controller.Profile(profile.ID).State; got != ProfileReady {
		t.Fatalf("expected ready state after stop, got %s", got)
	}
}

func TestControllerStopsProfileWhenSenderFails(t *testing.T) {
	sender := &fakeSender{err: errors.New("post message failed")}
	controller := NewController(Dependencies{Sender: sender, ResolveVirtualKey: fakeResolver, HoldDuration: time.Millisecond})
	profile, err := controller.CreateProfile()
	if err != nil {
		t.Fatal(err)
	}
	profile.Target = &WindowTarget{Handle: 100, Title: "Notepad"}
	profile.Bindings[0] = KeyBinding{Code: "KeyA", Label: "A", DelayMs: 1}
	if err := controller.StartProfile(profile.ID); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for controller.Profile(profile.ID).State != ProfileError && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond * 5)
	}
	if got := controller.Profile(profile.ID).State; got != ProfileError {
		t.Fatalf("expected error state, got %s", got)
	}
}
