package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"sg-tools/internal/platform/windows"
	"sg-tools/pkg/clicker"
	"sg-tools/pkg/windowsync"
)

type FollowSyncSnapshot struct {
	State         windowsync.State      `json:"state"`
	Main          *WindowInfo           `json:"main,omitempty"`
	Follows       []WindowInfo          `json:"follows"`
	Rules         clicker.KeyRuleConfig `json:"rules"`
	CapturedCount uint64                `json:"capturedCount"`
	LastCode      string                `json:"lastCode,omitempty"`
	LastError     string                `json:"lastError,omitempty"`
}

type FollowSyncStateEvent struct {
	State         windowsync.State `json:"state"`
	CapturedCount uint64           `json:"capturedCount"`
	LastCode      string           `json:"lastCode,omitempty"`
	Error         string           `json:"error,omitempty"`
}

type FollowSyncErrorEvent struct {
	Message string `json:"message"`
}

type FollowSyncWindowPickEvent struct {
	Kind   string      `json:"kind"`
	Role   string      `json:"role"`
	Target *WindowInfo `json:"target,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type syncWindowPicker interface {
	Begin(context.Context, clicker.WindowIdentity) (<-chan clicker.WindowPickEvent, error)
	Cancel() error
}

type FollowSyncService struct {
	mu     sync.RWMutex
	ctx    context.Context
	core   *windowsync.Service
	store  *clicker.ConfigStore
	picker syncWindowPicker
}

func NewFollowSyncService(configPath string) *FollowSyncService {
	service := &FollowSyncService{}
	service.core = windowsync.NewService(windowsync.Dependencies{
		Capture:        windows.NewInputCapture(),
		Sender:         windows.NewKeySender(0),
		ValidateTarget: windows.ValidateWindow,
		OnStateChanged: func(event windowsync.StateEvent) {
			service.emitState(event)
		},
	})
	service.store = clicker.NewConfigStore(configPath)
	service.picker = windows.NewWindowPicker()
	return service
}

func newFollowSyncServiceWithDeps(core *windowsync.Service, store *clicker.ConfigStore, picker syncWindowPicker) *FollowSyncService {
	return &FollowSyncService{core: core, store: store, picker: picker}
}

func (s *FollowSyncService) setContext(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
}

func (s *FollowSyncService) startup() error {
	config, err := s.store.Load()
	if err != nil {
		return fmt.Errorf("load follow-sync config: %w", err)
	}
	if err := s.core.Configure(windowsync.FilterConfig(config.FollowSync)); err != nil {
		_ = s.core.Configure(windowsync.FilterConfig{})
		return fmt.Errorf("invalid follow-sync rules: %w", err)
	}
	return nil
}

func (s *FollowSyncService) shutdown() {
	_ = s.picker.Cancel()
	s.core.Shutdown()
}

func (s *FollowSyncService) GetSnapshot() FollowSyncSnapshot {
	snapshot := s.core.Snapshot()
	result := FollowSyncSnapshot{
		State:         snapshot.State,
		Follows:       make([]WindowInfo, 0, len(snapshot.Follows)),
		Rules:         clicker.KeyRuleConfig{Include: append([]string(nil), snapshot.Filter.Include...), Exclude: append([]string(nil), snapshot.Filter.Exclude...)},
		CapturedCount: snapshot.CapturedCount,
		LastCode:      snapshot.LastCode,
		LastError:     snapshot.LastError,
	}
	if snapshot.Main != nil {
		info := windowInfo(*snapshot.Main)
		result.Main = &info
	}
	for _, follow := range snapshot.Follows {
		result.Follows = append(result.Follows, windowInfo(follow))
	}
	return result
}

func (s *FollowSyncService) SetRules(rules clicker.KeyRuleConfig) error {
	config, err := s.store.Load()
	if err != nil {
		return fmt.Errorf("load config before saving follow-sync rules: %w", err)
	}
	previous := config.FollowSync
	if err := s.core.Configure(windowsync.FilterConfig(rules)); err != nil {
		return err
	}
	config.FollowSync = rules
	if err := s.store.Save(config); err != nil {
		_ = s.core.Configure(windowsync.FilterConfig(previous))
		return err
	}
	return nil
}

func (s *FollowSyncService) BeginMainWindowPick() error {
	if s.core.Snapshot().State == windowsync.StateRunning {
		return windowsync.ErrSyncRunning
	}
	events, err := s.picker.Begin(context.Background(), clicker.WindowIdentity{})
	if err != nil {
		return err
	}
	go s.consumeWindowPick("main", events)
	return nil
}

func (s *FollowSyncService) BeginFollowWindowPick() error {
	if s.core.Snapshot().State == windowsync.StateRunning {
		return windowsync.ErrSyncRunning
	}
	events, err := s.picker.Begin(context.Background(), clicker.WindowIdentity{})
	if err != nil {
		return err
	}
	go s.consumeWindowPick("follow", events)
	return nil
}

func (s *FollowSyncService) CancelWindowPick() error {
	return s.picker.Cancel()
}

func (s *FollowSyncService) RemoveFollowWindow(index int) error {
	return s.core.RemoveFollowTarget(index)
}

func (s *FollowSyncService) ClearTargets() error {
	return s.core.ClearTargets()
}

func (s *FollowSyncService) Start() error { return s.core.Start() }

func (s *FollowSyncService) Stop() error { return s.core.Stop() }

func (s *FollowSyncService) consumeWindowPick(role string, events <-chan clicker.WindowPickEvent) {
	for event := range events {
		payload := FollowSyncWindowPickEvent{Kind: event.Kind, Role: role, Error: event.Error}
		if event.Target != nil {
			info := windowInfo(*event.Target)
			payload.Target = &info
		}
		if event.Kind != "complete" || event.Target == nil {
			s.emit("follow-sync:window-pick-"+event.Kind, payload)
			continue
		}
		var err error
		if role == "main" {
			err = s.core.SetMainTarget(*event.Target)
		} else {
			err = s.core.AddFollowTarget(*event.Target)
		}
		if err != nil {
			s.emitError(err)
			continue
		}
		s.emit("follow-sync:window-pick-complete", payload)
	}
}

func (s *FollowSyncService) emitState(event windowsync.StateEvent) {
	s.emit("follow-sync:state", FollowSyncStateEvent{State: event.State, CapturedCount: event.CapturedCount, LastCode: event.LastCode, Error: event.Error})
	if event.Error != "" {
		s.emitError(fmt.Errorf("%s", event.Error))
	}
}

func (s *FollowSyncService) emitError(err error) {
	s.emit("follow-sync:error", FollowSyncErrorEvent{Message: err.Error()})
}

func (s *FollowSyncService) emit(name string, payload any) {
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx != nil {
		runtime.EventsEmit(ctx, name, payload)
	}
}

func windowInfo(target clicker.WindowTarget) WindowInfo {
	return WindowInfo{Title: target.Title, ProcessName: target.ProcessName, PID: target.PID}
}
