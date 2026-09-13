package clicker

import (
	"container/heap"
	"context"
	"errors"
	"sort"
	"time"
)

type scheduleItem struct {
	next       time.Time
	index      int
	binding    KeyBinding
	virtualKey uint16
}

type scheduleHeap []scheduleItem

func (h scheduleHeap) Len() int { return len(h) }
func (h scheduleHeap) Less(i, j int) bool {
	if h[i].next.Equal(h[j].next) {
		return h[i].index < h[j].index
	}
	return h[i].next.Before(h[j].next)
}
func (h scheduleHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *scheduleHeap) Push(x any)   { *h = append(*h, x.(scheduleItem)) }
func (h *scheduleHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func (c *Controller) StartProfile(id string) error {
	c.mu.Lock()
	profile, ok := c.profiles[id]
	if !ok {
		c.mu.Unlock()
		return ErrProfileMissing
	}
	if profile.State == ProfileRunning {
		c.mu.Unlock()
		return nil
	}
	if err := profile.ValidateForStart(); err != nil {
		c.mu.Unlock()
		return err
	}
	if c.sender == nil {
		c.mu.Unlock()
		return errors.New("key sender is unavailable")
	}
	target := *profile.Target
	bindings := make([]KeyBinding, 0, MaxBindings)
	virtualKeys := make([]uint16, 0, MaxBindings)
	for _, binding := range profile.Bindings {
		if binding.Configured() {
			bindings = append(bindings, binding)
			if c.resolveVirtualKey == nil {
				c.mu.Unlock()
				return errors.New("virtual-key resolver is unavailable")
			}
			virtualKey, err := c.resolveVirtualKey(binding.Code)
			if err != nil {
				c.mu.Unlock()
				return err
			}
			virtualKeys = append(virtualKeys, virtualKey)
		}
	}
	profile.State = ProfileRunning
	profile.LastError = ""
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	c.cancel[id] = cancel
	c.done[id] = done
	c.mu.Unlock()
	c.notify(StateEvent{ProfileID: id, State: ProfileRunning})

	go c.runProfile(ctx, id, target, bindings, virtualKeys, done)
	return nil
}

func (c *Controller) runProfile(ctx context.Context, id string, target WindowTarget, bindings []KeyBinding, virtualKeys []uint16, done chan struct{}) {
	defer close(done)
	h := make(scheduleHeap, 0, len(bindings))
	now := time.Now()
	for index, binding := range bindings {
		heap.Push(&h, scheduleItem{next: now, index: index, binding: binding, virtualKey: virtualKeys[index]})
	}

	for {
		item := heap.Pop(&h).(scheduleItem)
		if err := waitUntil(ctx, item.next); err != nil {
			c.finishProfile(id, nil)
			return
		}
		if err := c.sender.Press(ctx, target, item.virtualKey); err != nil {
			c.finishProfile(id, err)
			return
		}
		item.next = time.Now().Add(time.Duration(item.binding.DelayMs) * time.Millisecond)
		heap.Push(&h, item)
	}
}

func waitUntil(ctx context.Context, target time.Time) error {
	delay := time.Until(target)
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Controller) finishProfile(id string, runErr error) {
	c.mu.Lock()
	profile, ok := c.profiles[id]
	if !ok {
		c.mu.Unlock()
		return
	}
	if runErr != nil {
		profile.State = ProfileError
		profile.LastError = runErr.Error()
		event := StateEvent{ProfileID: id, State: profile.State, Error: profile.LastError}
		c.mu.Unlock()
		c.notify(event)
		return
	}
	if profile.State == ProfileRunning {
		profile.State = ProfileReady
	}
	event := StateEvent{ProfileID: id, State: profile.State, Error: profile.LastError}
	c.mu.Unlock()
	c.notify(event)
}

func (c *Controller) StopProfile(id string) error {
	c.mu.Lock()
	profile, ok := c.profiles[id]
	if !ok {
		c.mu.Unlock()
		return ErrProfileMissing
	}
	cancel := c.cancel[id]
	done := c.done[id]
	if profile.State == ProfileRunning {
		profile.State = ProfileReady
		profile.LastError = ""
	}
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	c.notify(StateEvent{ProfileID: id, State: c.profileState(id)})
	return nil
}

func (c *Controller) profileState(id string) ProfileState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if profile, ok := c.profiles[id]; ok {
		return profile.State
	}
	return ProfileIdle
}

func (c *Controller) notify(event StateEvent) {
	if c.onStateChanged != nil {
		c.onStateChanged(event)
	}
}

func (c *Controller) CloseProfile(id string) error {
	if err := c.StopProfile(id); err != nil && !errors.Is(err, ErrProfileMissing) {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.profiles[id]; !ok {
		return ErrProfileMissing
	}
	delete(c.profiles, id)
	delete(c.cancel, id)
	delete(c.done, id)
	if c.activeID == id {
		c.activeID = ""
		ids := make([]string, 0, len(c.profiles))
		for profileID := range c.profiles {
			ids = append(ids, profileID)
		}
		sort.Strings(ids)
		if len(ids) > 0 {
			c.activeID = ids[0]
		}
	}
	return nil
}

type BatchSkip struct {
	ProfileID string `json:"profileId"`
	Reason    string `json:"reason"`
}

type BatchResult struct {
	Started []string    `json:"started"`
	Skipped []BatchSkip `json:"skipped"`
}

func (c *Controller) StartAll() BatchResult {
	ids := c.profileIDs()
	result := BatchResult{}
	for _, id := range ids {
		if err := c.StartProfile(id); err != nil {
			result.Skipped = append(result.Skipped, BatchSkip{ProfileID: id, Reason: err.Error()})
			continue
		}
		result.Started = append(result.Started, id)
	}
	return result
}

func (c *Controller) StopAll() {
	for _, id := range c.profileIDs() {
		_ = c.StopProfile(id)
	}
}

func (c *Controller) profileIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]string, 0, len(c.profiles))
	for id := range c.profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (c *Controller) Shutdown() {
	c.StopAll()
	if c.picker != nil {
		_ = c.picker.Cancel()
	}
	if c.hotkeys != nil {
		_ = c.hotkeys.Unregister()
	}
}
