package windowsync

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"sg-tools/pkg/clicker"
)

type Service struct {
	mu             sync.RWMutex
	capture        InputCapture
	sender         clicker.KeyEventSender
	validateTarget func(clicker.WindowTarget) error
	onStateChanged func(StateEvent)
	filter         FilterConfig
	main           *clicker.WindowTarget
	follows        []clicker.WindowTarget
	state          State
	lastError      string
	capturedCount  uint64
	lastCode       string
	cancel         context.CancelFunc
	done           chan struct{}
}

func NewService(deps Dependencies) *Service {
	return &Service{
		capture:        deps.Capture,
		sender:         deps.Sender,
		validateTarget: deps.ValidateTarget,
		onStateChanged: deps.OnStateChanged,
		state:          StateIdle,
	}
}

func (s *Service) Configure(filter FilterConfig) error {
	if err := filter.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateRunning {
		return ErrSyncRunning
	}
	s.filter = cloneFilter(filter)
	return nil
}

func (s *Service) SetMainTarget(target clicker.WindowTarget) error {
	if target.Handle == 0 {
		return ErrNoMainWindow
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateRunning {
		return ErrSyncRunning
	}
	for _, follow := range s.follows {
		if follow.Handle == target.Handle {
			return fmt.Errorf("%w: main target is already a follow window", ErrTargetInvalid)
		}
	}
	copyTarget := target
	s.main = &copyTarget
	s.lastError = ""
	s.updateReadyLocked()
	return nil
}

func (s *Service) SetFollowTargets(targets []clicker.WindowTarget) error {
	if err := validateFollowTargets(targets); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateRunning {
		return ErrSyncRunning
	}
	if s.main != nil {
		for _, target := range targets {
			if target.Handle == s.main.Handle {
				return fmt.Errorf("%w: follow target is the main window", ErrTargetInvalid)
			}
		}
	}
	s.follows = cloneTargets(targets)
	s.lastError = ""
	s.updateReadyLocked()
	return nil
}

func (s *Service) AddFollowTarget(target clicker.WindowTarget) error {
	if target.Handle == 0 {
		return ErrTargetInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateRunning {
		return ErrSyncRunning
	}
	if s.main != nil && s.main.Handle == target.Handle {
		return fmt.Errorf("%w: follow target is the main window", ErrTargetInvalid)
	}
	for _, existing := range s.follows {
		if existing.Handle == target.Handle {
			return fmt.Errorf("%w: duplicate follow target", ErrTargetInvalid)
		}
	}
	s.follows = append(s.follows, target)
	s.lastError = ""
	s.updateReadyLocked()
	return nil
}

func (s *Service) RemoveFollowTarget(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateRunning {
		return ErrSyncRunning
	}
	if index < 0 || index >= len(s.follows) {
		return ErrFollowWindowMissing
	}
	s.follows = append(s.follows[:index], s.follows[index+1:]...)
	s.updateReadyLocked()
	return nil
}

func (s *Service) ClearTargets() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateRunning {
		return ErrSyncRunning
	}
	s.main = nil
	s.follows = nil
	s.lastError = ""
	s.updateReadyLocked()
	return nil
}

func (s *Service) Start() error {
	s.mu.Lock()
	if s.state == StateRunning {
		s.mu.Unlock()
		return nil
	}
	if s.main == nil || s.main.Handle == 0 {
		s.mu.Unlock()
		return ErrNoMainWindow
	}
	if len(s.follows) == 0 {
		s.mu.Unlock()
		return ErrNoFollowWindows
	}
	if err := s.filter.Validate(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.capture == nil {
		s.mu.Unlock()
		return ErrCaptureUnavailable
	}
	if s.sender == nil {
		s.mu.Unlock()
		return ErrSenderUnavailable
	}
	main := *s.main
	follows := cloneTargets(s.follows)
	filter := cloneFilter(s.filter)
	validateTarget := s.validateTarget
	s.mu.Unlock()

	if validateTarget != nil {
		if err := validateTarget(main); err != nil {
			return fmt.Errorf("%w: main window: %v", ErrTargetInvalid, err)
		}
		for _, follow := range follows {
			if err := validateTarget(follow); err != nil {
				return fmt.Errorf("%w: follow window: %v", ErrTargetInvalid, err)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	events, err := s.capture.Start(ctx, CaptureSpec{Main: main})
	if err != nil {
		cancel()
		return err
	}
	done := make(chan struct{})
	s.mu.Lock()
	s.state = StateRunning
	s.lastError = ""
	s.cancel = cancel
	s.done = done
	s.mu.Unlock()
	s.notify()
	go s.run(ctx, cancel, done, events, main, follows, filter)
	return nil
}

func (s *Service) run(ctx context.Context, cancel context.CancelFunc, done chan struct{}, events <-chan InputEvent, main clicker.WindowTarget, follows []clicker.WindowTarget, filter FilterConfig) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				if ctx.Err() == nil {
					s.fail(errors.New("input capture stopped unexpectedly"), cancel)
				}
				return
			}
			if event.Kind != InputKindKeyboard || (event.SourceHandle != 0 && event.SourceHandle != main.Handle) {
				continue
			}
			s.mu.Lock()
			if s.state != StateRunning {
				s.mu.Unlock()
				return
			}
			s.capturedCount++
			s.lastCode = event.Code
			s.mu.Unlock()
			if !filter.Allows(event.Code) {
				continue
			}
			for _, follow := range follows {
				if err := s.sender.SendKey(ctx, follow, event.Transition); err != nil {
					s.fail(fmt.Errorf("%w: %v", ErrTargetInvalid, err), cancel)
					return
				}
			}
			s.notify()
		}
	}
}

func (s *Service) fail(err error, cancel context.CancelFunc) {
	s.mu.Lock()
	if s.state != StateRunning {
		s.mu.Unlock()
		return
	}
	s.state = StateError
	s.lastError = err.Error()
	s.mu.Unlock()
	cancel()
	_ = s.capture.Stop()
	s.notify()
}

func (s *Service) Stop() error {
	s.mu.Lock()
	if s.state != StateRunning {
		s.mu.Unlock()
		return nil
	}
	cancel, done := s.cancel, s.done
	s.state = StateReady
	s.lastError = ""
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if s.capture != nil {
		_ = s.capture.Stop()
	}
	if done != nil {
		<-done
	}
	s.mu.Lock()
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()
	s.notify()
	return nil
}

func (s *Service) Shutdown() {
	_ = s.Stop()
	if s.capture != nil {
		_ = s.capture.Stop()
	}
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var main *clicker.WindowTarget
	if s.main != nil {
		copyTarget := *s.main
		main = &copyTarget
	}
	return Snapshot{
		State:         s.state,
		Main:          main,
		Follows:       cloneTargets(s.follows),
		Filter:        cloneFilter(s.filter),
		CapturedCount: s.capturedCount,
		LastCode:      s.lastCode,
		LastError:     s.lastError,
	}
}

func (s *Service) updateReadyLocked() {
	if s.main != nil && len(s.follows) > 0 {
		s.state = StateReady
	} else {
		s.state = StateIdle
	}
}

func (s *Service) notify() {
	if s.onStateChanged == nil {
		return
	}
	snapshot := s.Snapshot()
	s.onStateChanged(StateEvent{State: snapshot.State, CapturedCount: snapshot.CapturedCount, LastCode: snapshot.LastCode, Error: snapshot.LastError})
}

func validateFollowTargets(targets []clicker.WindowTarget) error {
	seen := make(map[uintptr]struct{}, len(targets))
	for _, target := range targets {
		if target.Handle == 0 {
			return ErrTargetInvalid
		}
		if _, exists := seen[target.Handle]; exists {
			return fmt.Errorf("%w: duplicate follow target", ErrTargetInvalid)
		}
		seen[target.Handle] = struct{}{}
	}
	return nil
}

func cloneTargets(targets []clicker.WindowTarget) []clicker.WindowTarget {
	return append([]clicker.WindowTarget(nil), targets...)
}

func cloneFilter(filter FilterConfig) FilterConfig {
	return FilterConfig{Include: append([]string(nil), filter.Include...), Exclude: append([]string(nil), filter.Exclude...)}
}
