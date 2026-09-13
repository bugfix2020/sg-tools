package clicker

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MaxProfiles = 8
	MaxBindings = 12
)

var (
	ErrWindowNotBound      = errors.New("window is not bound")
	ErrNoEnabledKeys       = errors.New("no enabled keys")
	ErrProfileRunning      = errors.New("profile is running")
	ErrProfileLimit        = errors.New("profile limit reached")
	ErrProfileMissing      = errors.New("profile not found")
	ErrInvalidBinding      = errors.New("invalid key binding")
	ErrInvalidHotkey       = errors.New("invalid hotkey")
	ErrDuplicateHotkey     = errors.New("duplicate hotkey")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

type KeyBinding struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	DelayMs uint32 `json:"delayMs"`
}

func (b KeyBinding) Configured() bool {
	return strings.TrimSpace(b.Code) != ""
}

func (b KeyBinding) Validate() error {
	if !b.Configured() {
		return ErrInvalidBinding
	}
	if b.DelayMs > 9_999_999 {
		return fmt.Errorf("delay must be between 0 and 9999999 milliseconds")
	}
	return nil
}

type WindowTarget struct {
	Handle      uintptr `json:"-"`
	Title       string  `json:"title"`
	ProcessName string  `json:"processName"`
	PID         uint32  `json:"pid"`
}

type WindowIdentity struct {
	Handle uintptr
}

type WindowPickEvent struct {
	Kind   string        `json:"kind"`
	Target *WindowTarget `json:"target,omitempty"`
	Error  string        `json:"error,omitempty"`
}

type StateEvent struct {
	ProfileID string
	State     ProfileState
	Error     string
}

type ProfileState string

const (
	ProfileIdle    ProfileState = "idle"
	ProfilePicking ProfileState = "picking"
	ProfileReady   ProfileState = "ready"
	ProfileRunning ProfileState = "running"
	ProfileError   ProfileState = "error"
)

type Profile struct {
	ID        string
	Name      string
	Bindings  [MaxBindings]KeyBinding
	Target    *WindowTarget
	State     ProfileState
	LastError string
}

func NewProfile(id, name string) *Profile {
	return &Profile{ID: id, Name: name, State: ProfileIdle}
}

func (p *Profile) ValidateForStart() error {
	if p.Target == nil || p.Target.Handle == 0 {
		return ErrWindowNotBound
	}
	configured := false
	for _, binding := range p.Bindings {
		if binding.Configured() {
			configured = true
			if err := binding.Validate(); err != nil {
				return err
			}
		}
	}
	if configured {
		return nil
	}
	return ErrNoEnabledKeys
}

type PressRecord struct {
	Target     WindowTarget
	VirtualKey uint16
}

type KeySender interface {
	// Press owns the complete key-down/hold/key-up lifecycle for one press.
	Press(context.Context, WindowTarget, uint16) error
}

type VirtualKeyResolver func(string) (uint16, error)

type WindowPicker interface {
	Begin(context.Context, WindowIdentity) (<-chan WindowPickEvent, error)
	Cancel() error
}

type HotkeyManager interface {
	Register(HotkeyConfig) error
	Unregister() error
}

type Dependencies struct {
	Sender            KeySender
	ResolveVirtualKey VirtualKeyResolver
	Picker            WindowPicker
	Hotkeys           HotkeyManager
	HoldDuration      time.Duration
	OnStateChanged    func(StateEvent)
}

type Controller struct {
	mu                sync.RWMutex
	sender            KeySender
	resolveVirtualKey VirtualKeyResolver
	picker            WindowPicker
	hotkeys           HotkeyManager
	holdDuration      time.Duration
	onStateChanged    func(StateEvent)
	profiles          map[string]*Profile
	cancel            map[string]context.CancelFunc
	done              map[string]chan struct{}
	activeID          string
	nextID            int
}

func NewController(deps Dependencies) *Controller {
	hold := deps.HoldDuration
	if hold <= 0 {
		hold = 5 * time.Millisecond
	}
	return &Controller{
		sender:            deps.Sender,
		resolveVirtualKey: deps.ResolveVirtualKey,
		picker:            deps.Picker,
		hotkeys:           deps.Hotkeys,
		holdDuration:      hold,
		onStateChanged:    deps.OnStateChanged,
		profiles:          make(map[string]*Profile),
		cancel:            make(map[string]context.CancelFunc),
		done:              make(map[string]chan struct{}),
	}
}

func (c *Controller) CreateProfile() (*Profile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.profiles) >= MaxProfiles {
		return nil, ErrProfileLimit
	}
	c.nextID++
	id := fmt.Sprintf("profile-%d", c.nextID)
	profile := NewProfile(id, fmt.Sprintf("Tab %d", len(c.profiles)+1))
	c.profiles[id] = profile
	if c.activeID == "" {
		c.activeID = id
	}
	return profile, nil
}

func (c *Controller) AddProfile(profile *Profile) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.profiles) >= MaxProfiles {
		return ErrProfileLimit
	}
	if profile == nil || profile.ID == "" {
		return ErrProfileMissing
	}
	if _, exists := c.profiles[profile.ID]; exists {
		return fmt.Errorf("profile %q already exists", profile.ID)
	}
	if profile.State == "" {
		profile.State = ProfileIdle
	}
	c.profiles[profile.ID] = profile
	if strings.HasPrefix(profile.ID, "profile-") {
		if id, err := strconv.Atoi(strings.TrimPrefix(profile.ID, "profile-")); err == nil && id > c.nextID {
			c.nextID = id
		}
	}
	if c.activeID == "" {
		c.activeID = profile.ID
	}
	return nil
}

func (c *Controller) Profile(id string) *Profile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.profiles[id]
}

func (c *Controller) Profiles() []*Profile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*Profile, 0, len(c.profiles))
	for _, profile := range c.profiles {
		result = append(result, profile)
	}
	return result
}

func (c *Controller) Snapshot() ([]Profile, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Profile, 0, len(c.profiles))
	for _, profile := range c.profiles {
		copyProfile := *profile
		copyProfile.Bindings = profile.Bindings
		if profile.Target != nil {
			target := *profile.Target
			copyProfile.Target = &target
		}
		result = append(result, copyProfile)
	}
	return result, c.activeID
}

func (c *Controller) RenameProfile(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	profile, ok := c.profiles[id]
	if !ok {
		return ErrProfileMissing
	}
	if profile.State == ProfileRunning {
		return ErrProfileRunning
	}
	profile.Name = name
	return nil
}

func (c *Controller) SetActiveProfile(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.profiles[id]; !ok {
		return ErrProfileMissing
	}
	c.activeID = id
	return nil
}

func (c *Controller) ActiveProfile() *Profile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.profiles[c.activeID]
}

func (c *Controller) SetTarget(id string, target WindowTarget) error {
	c.mu.Lock()
	profile, ok := c.profiles[id]
	if !ok {
		c.mu.Unlock()
		return ErrProfileMissing
	}
	if profile.State == ProfileRunning {
		c.mu.Unlock()
		return ErrProfileRunning
	}
	profile.Target = &target
	profile.LastError = ""
	profile.State = ProfileReady
	event := StateEvent{ProfileID: id, State: profile.State}
	c.mu.Unlock()
	c.notify(event)
	return nil
}

func (c *Controller) SetPicking(id string, picking bool) error {
	c.mu.Lock()
	profile, ok := c.profiles[id]
	if !ok {
		c.mu.Unlock()
		return ErrProfileMissing
	}
	if profile.State == ProfileRunning {
		c.mu.Unlock()
		return ErrProfileRunning
	}
	if picking {
		profile.State = ProfilePicking
	} else if profile.Target != nil {
		profile.State = ProfileReady
	} else {
		profile.State = ProfileIdle
	}
	event := StateEvent{ProfileID: id, State: profile.State, Error: profile.LastError}
	c.mu.Unlock()
	c.notify(event)
	return nil
}

func (c *Controller) SetBinding(id string, index int, binding KeyBinding) error {
	if index < 0 || index >= MaxBindings {
		return ErrInvalidBinding
	}
	if binding.Configured() {
		if err := binding.Validate(); err != nil {
			return err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	profile, ok := c.profiles[id]
	if !ok {
		return ErrProfileMissing
	}
	if profile.State == ProfileRunning {
		return ErrProfileRunning
	}
	profile.Bindings[index] = binding
	if profile.Target != nil {
		profile.State = ProfileReady
	}
	return nil
}

func (c *Controller) ClearBinding(id string, index int) error {
	return c.SetBinding(id, index, KeyBinding{})
}
