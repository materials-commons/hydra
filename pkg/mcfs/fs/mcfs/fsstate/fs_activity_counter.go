package fsstate

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/materials-commons/hydra/pkg/ctx"
)

// ActivityCounter tracks activity against an object. It atomically updates activityCount. The
// LastSeenActivityCount and LastChanged are meant to be used by a monitor for tracking how
// activity has changed.
type ActivityCounter struct {
	activityCount         atomic.Uint64
	lastSeenActivityCount atomic.Uint64
	lastChanged           time.Time

	// Protects LastChanged
	mu sync.RWMutex
}

// NewActivityCounter creates a new ActivityCounter with LastChanged set to the current time.
func NewActivityCounter() *ActivityCounter {
	return &ActivityCounter{
		lastChanged: time.Now(),
	}
}

// IncrementActivityCount atomically updates the activityCount
func (c *ActivityCounter) IncrementActivityCount() {
	c.activityCount.Add(1)
}

func (c *ActivityCounter) GetActivityCount() uint64 {
	return c.activityCount.Load()
}

func (c *ActivityCounter) GetLastSeenActivityCount() uint64 {
	return c.lastSeenActivityCount.Load()
}

func (c *ActivityCounter) SetLastSeenActivityCount(val uint64) {
	c.lastSeenActivityCount.Store(val)
}

func (c *ActivityCounter) GetLastChanged() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastChanged
}

func (c *ActivityCounter) SetLastChanged(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastChanged = t
}

type ExpiredActivityHandlerFN func(activityKey string)

type ActivityCounterMonitor struct {
	activityCounters       sync.Map
	inactivityDuration     time.Duration
	expiredActivityHandler ExpiredActivityHandlerFN
}

func NewActivityCounterMonitor(inactivityDuration time.Duration) *ActivityCounterMonitor {
	return &ActivityCounterMonitor{
		inactivityDuration: inactivityDuration,
	}
}

func (m *ActivityCounterMonitor) Start(ctx context.Context) {
	for {
		if canceled := m.loadAndCheckEachActivityCounter(ctx); canceled {
			break
		}

		select {
		case <-ctx.Done():
			break
		case <-time.After(20 * time.Second):
		}
	}
}

func (m *ActivityCounterMonitor) loadAndCheckEachActivityCounter(c context.Context) bool {
	isDone := false
	now := time.Now()

	m.activityCounters.Range(func(key, value any) bool {
		if ctx.IsDone(c) {
			isDone = true
			return false
		}

		ac, ok := value.(*ActivityCounter)
		if !ok {
			return true
		}
		currentActivityCount := ac.GetActivityCount()
		lastSeenActivityCount := ac.GetLastSeenActivityCount()

		switch {
		case currentActivityCount == lastSeenActivityCount:
			lastChanged := ac.GetLastChanged()
			allowedInactive := lastChanged.Add(m.inactivityDuration)
			if now.After(allowedInactive) {
				m.expiredActivityHandler(key.(string))
			}

		default:
			ac.SetLastChanged(now)
			ac.SetLastSeenActivityCount(currentActivityCount)
		}

		return true
	})

	return isDone
}

func (m *ActivityCounterMonitor) GetOrCreateActivityCounter(key string) *ActivityCounter {
	activityCounter := NewActivityCounter()
	ac, _ := m.activityCounters.LoadOrStore(key, activityCounter)

	return ac.(*ActivityCounter)
}

func (m *ActivityCounterMonitor) GetActivityCounter(key string) *ActivityCounter {
	activityCounter, found := m.activityCounters.Load(key)
	if !found {
		return nil
	}

	return activityCounter.(*ActivityCounter)
}

func (m *ActivityCounterMonitor) ForEach(fn func(key string, ac *ActivityCounter)) {
	m.activityCounters.Range(func(key, value any) bool {
		kstr := key.(string)
		ac := value.(*ActivityCounter)
		fn(kstr, ac)
		return true
	})
}
