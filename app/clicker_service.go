package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"sg-tools/internal/platform/windows"
	"sg-tools/pkg/clicker"
)

type WindowInfo struct {
	Title       string `json:"title"`
	ProcessName string `json:"processName"`
	PID         uint32 `json:"pid"`
}

type ProfileView struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Bindings  []clicker.KeyBinding `json:"bindings"`
	Target    *WindowInfo          `json:"target,omitempty"`
	State     clicker.ProfileState `json:"state"`
	LastError string               `json:"lastError,omitempty"`
}

type Snapshot struct {
	Profiles        []ProfileView        `json:"profiles"`
	ActiveProfileID string               `json:"activeProfileId"`
	Hotkeys         clicker.HotkeyConfig `json:"hotkeys"`
}

type ProfileStateEvent struct {
	ProfileID string               `json:"profileId"`
	State     clicker.ProfileState `json:"state"`
	Error     string               `json:"error,omitempty"`
}

type WindowPickEvent struct {
	Kind      string      `json:"kind"`
	ProfileID string      `json:"profileId"`
	Target    *WindowInfo `json:"target,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type ClickerService struct {
	mu           sync.RWMutex
	ctx          context.Context
	controller   *clicker.Controller
	store        *clicker.ConfigStore
	hotkeys      *windows.HotkeyManager
	picker       *windows.WindowPicker
	hotkeyConfig clicker.HotkeyConfig
	soundDir     string
}

func NewClickerService(configPath, soundDir string) *ClickerService {
	var service *ClickerService
	picker := windows.NewWindowPicker()
	var hotkeys *windows.HotkeyManager
	hotkeys = windows.NewHotkeyManager(func(action string) {
		if service != nil {
			service.handleHotkey(action)
		}
	})
	controller := clicker.NewController(clicker.Dependencies{
		Sender:            windows.NewKeySender(5 * time.Millisecond),
		ResolveVirtualKey: windows.ResolveVirtualKey,
		Picker:            picker,
		Hotkeys:           hotkeys,
		OnStateChanged: func(event clicker.StateEvent) {
			if service != nil {
				service.emitProfileState(event)
			}
		},
	})
	service = &ClickerService{
		controller:   controller,
		store:        clicker.NewConfigStore(configPath),
		hotkeys:      hotkeys,
		picker:       picker,
		hotkeyConfig: clicker.DefaultHotkeys(),
		soundDir:     soundDir,
	}
	return service
}

func (s *ClickerService) setContext(ctx context.Context) { s.mu.Lock(); s.ctx = ctx; s.mu.Unlock() }

func (s *ClickerService) startup() error {
	config, err := s.store.Load()
	if err != nil {
		config = clicker.DefaultConfig()
		if saveErr := s.store.Save(config); saveErr != nil {
			return fmt.Errorf("restore default config: %w", saveErr)
		}
		s.emitError("", fmt.Errorf("配置加载失败，已恢复默认配置：%w", err))
	}
	s.mu.Lock()
	s.hotkeyConfig = config.Hotkeys
	s.mu.Unlock()
	for _, persisted := range config.Profiles {
		profile := clicker.NewProfile(persisted.ID, persisted.Name)
		for index := 0; index < clicker.MaxBindings && index < len(persisted.Bindings); index++ {
			profile.Bindings[index] = persisted.Bindings[index]
		}
		if err := s.controller.AddProfile(profile); err != nil {
			break
		}
	}
	if len(s.controller.Profiles()) == 0 {
		if _, err := s.controller.CreateProfile(); err != nil {
			return err
		}
	}
	if err := s.hotkeys.Register(config.Hotkeys); err != nil {
		s.emitError("", err)
		return err
	}
	return nil
}

func (s *ClickerService) shutdown() {
	s.controller.Shutdown()
}

func (s *ClickerService) GetSnapshot() Snapshot {
	profiles, activeID := s.controller.Snapshot()
	s.mu.RLock()
	hotkeys := s.hotkeyConfig
	s.mu.RUnlock()
	views := make([]ProfileView, 0, len(profiles))
	for _, profile := range profiles {
		views = append(views, profileView(profile))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return Snapshot{Profiles: views, ActiveProfileID: activeID, Hotkeys: hotkeys}
}

func (s *ClickerService) CreateProfile() (ProfileView, error) {
	profile, err := s.controller.CreateProfile()
	if err != nil {
		return ProfileView{}, err
	}
	if err := s.save(); err != nil {
		return ProfileView{}, err
	}
	return profileView(*profile), nil
}

func (s *ClickerService) CloseProfile(profileID string) error {
	if err := s.controller.CloseProfile(profileID); err != nil {
		return err
	}
	return s.save()
}

func (s *ClickerService) RenameProfile(profileID, name string) error {
	if err := s.controller.RenameProfile(profileID, name); err != nil {
		return err
	}
	return s.save()
}

func (s *ClickerService) SetBinding(profileID string, index int, binding clicker.KeyBinding) error {
	if err := s.controller.SetBinding(profileID, index, binding); err != nil {
		return err
	}
	return s.save()
}

func (s *ClickerService) ClearBinding(profileID string, index int) error {
	if err := s.controller.ClearBinding(profileID, index); err != nil {
		return err
	}
	return s.save()
}

func (s *ClickerService) BeginWindowPick(profileID string) error {
	if err := s.controller.SetPicking(profileID, true); err != nil {
		return err
	}
	events, err := s.picker.Begin(context.Background(), clicker.WindowIdentity{})
	if err != nil {
		_ = s.controller.SetPicking(profileID, false)
		return err
	}
	go func() {
		for event := range events {
			s.emitWindowPick(profileID, event)
			if event.Kind == "complete" && event.Target != nil {
				if err := s.controller.SetTarget(profileID, *event.Target); err != nil {
					s.emitError(profileID, err)
				} else {
					_ = s.save()
				}
			} else if event.Kind == "cancelled" {
				_ = s.controller.SetPicking(profileID, false)
			}
		}
	}()
	return nil
}

func (s *ClickerService) CancelWindowPick() error { return s.picker.Cancel() }

func (s *ClickerService) SetActiveProfile(profileID string) error {
	return s.controller.SetActiveProfile(profileID)
}

func (s *ClickerService) StartProfile(profileID string) error {
	if err := s.controller.StartProfile(profileID); err != nil {
		return err
	}
	s.playSound("start.wav")
	return nil
}

func (s *ClickerService) StopProfile(profileID string) error {
	if err := s.controller.StopProfile(profileID); err != nil {
		return err
	}
	s.playSound("stop.wav")
	return nil
}

func (s *ClickerService) StartAll() clicker.BatchResult {
	result := s.controller.StartAll()
	if len(result.Started) > 0 {
		s.playSound("start.wav")
	}
	return result
}

func (s *ClickerService) StopAll() {
	s.controller.StopAll()
	s.playSound("stop.wav")
}

func (s *ClickerService) GetHotkeys() clicker.HotkeyConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hotkeyConfig
}

func (s *ClickerService) SetHotkeys(config clicker.HotkeyConfig) error {
	if err := clicker.ValidateHotkeys(config); err != nil {
		return err
	}
	old := s.GetHotkeys()
	if err := s.hotkeys.Register(config); err != nil {
		return err
	}
	s.mu.Lock()
	s.hotkeyConfig = config
	s.mu.Unlock()
	if err := s.save(); err != nil {
		_ = s.hotkeys.Register(old)
		s.mu.Lock()
		s.hotkeyConfig = old
		s.mu.Unlock()
		return err
	}
	s.emit("clicker:hotkeys-changed", config)
	return nil
}

func (s *ClickerService) ResetHotkeys() (clicker.HotkeyConfig, error) {
	config := clicker.DefaultHotkeys()
	if err := s.SetHotkeys(config); err != nil {
		return clicker.HotkeyConfig{}, err
	}
	return config, nil
}

func (s *ClickerService) RestartAsAdministrator() error {
	return windows.RestartAsAdministrator()
}

func (s *ClickerService) handleHotkey(action string) {
	switch action {
	case "active-start":
		if profile := s.controller.ActiveProfile(); profile != nil {
			_ = s.StartProfile(profile.ID)
		}
	case "active-stop":
		if profile := s.controller.ActiveProfile(); profile != nil {
			_ = s.StopProfile(profile.ID)
		}
	case "global-start":
		_ = s.StartAll()
	case "global-stop":
		s.StopAll()
	}
}

func (s *ClickerService) save() error {
	profiles, _ := s.controller.Snapshot()
	followSync := clicker.KeyRuleConfig{}
	if existing, err := s.store.Load(); err == nil {
		followSync = existing.FollowSync
	}
	config := clicker.AppConfig{Version: 1, Hotkeys: s.GetHotkeys(), Profiles: make([]clicker.PersistedProfile, 0, len(profiles)), FollowSync: followSync}
	for _, profile := range profiles {
		bindings := make([]clicker.KeyBinding, len(profile.Bindings))
		copy(bindings, profile.Bindings[:])
		config.Profiles = append(config.Profiles, clicker.PersistedProfile{ID: profile.ID, Name: profile.Name, Bindings: bindings})
	}
	return s.store.Save(config)
}

func (s *ClickerService) emitProfileState(event clicker.StateEvent) {
	s.emit("clicker:profile-state", ProfileStateEvent{ProfileID: event.ProfileID, State: event.State, Error: event.Error})
	if event.Error != "" {
		s.emitError(event.ProfileID, fmt.Errorf("%s", event.Error))
	}
}

func (s *ClickerService) emitWindowPick(profileID string, event clicker.WindowPickEvent) {
	payload := WindowPickEvent{Kind: event.Kind, ProfileID: profileID, Error: event.Error}
	if event.Target != nil {
		payload.Target = &WindowInfo{Title: event.Target.Title, ProcessName: event.Target.ProcessName, PID: event.Target.PID}
	}
	s.emit("clicker:window-pick-"+event.Kind, payload)
}

func (s *ClickerService) emitError(profileID string, err error) {
	s.emit("clicker:error", map[string]string{"profileId": profileID, "message": err.Error()})
}

func (s *ClickerService) emit(name string, payload any) {
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx != nil {
		runtime.EventsEmit(ctx, name, payload)
	}
}

func (s *ClickerService) playSound(name string) {
	if s.soundDir == "" {
		return
	}
	_ = windows.PlaySound(filepath.Join(s.soundDir, name))
}

func profileView(profile clicker.Profile) ProfileView {
	bindings := make([]clicker.KeyBinding, len(profile.Bindings))
	copy(bindings, profile.Bindings[:])
	view := ProfileView{ID: profile.ID, Name: profile.Name, Bindings: bindings, State: profile.State, LastError: profile.LastError}
	if profile.Target != nil {
		view.Target = &WindowInfo{Title: profile.Target.Title, ProcessName: profile.Target.ProcessName, PID: profile.Target.PID}
	}
	return view
}
