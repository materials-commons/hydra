package mcfs

import (
	"sync"
	"sync/atomic"
	"time"
)

// ActivityCounter tracks activity against an object. It atomically updates activityCount. The
// LastSeenActivityCount and LastChanged are meant to be used by a monitor for tracking how
// activity has changed.
type ActivityCounter struct {
	activityCount         atomic.Uint64
	LastSeenActivityCount uint64
	LastChanged           time.Time
}

// NewActivityCounter creates a new ActivityCounter with LastChanged set to the current time.
func NewActivityCounter() *ActivityCounter {
	return &ActivityCounter{
		LastChanged: time.Now(),
	}
}

// IncrementActivityCount atomically updates the activityCount
func (c *ActivityCounter) IncrementActivityCount() {
	c.activityCount.Add(1)
}

func (c *ActivityCounter) GetActivityCount() uint64 {
	return c.activityCount.Load()
}

type ActivityCounterMonitor struct {
	activityCounters   map[string]*ActivityCounter
	inactivityDuration time.Duration
	mu                 sync.Mutex
}

func NewActivityCounterMonitor(inactivityDuration time.Duration) *ActivityCounterMonitor {
	return &ActivityCounterMonitor{
		activityCounters:   make(map[string]*ActivityCounter),
		inactivityDuration: inactivityDuration,
	}
}

func (m *ActivityCounterMonitor) Start() {

}

func (m *ActivityCounterMonitor) GetOrCreateActivityCounter(key string) *ActivityCounter {
	m.mu.Lock()
	defer m.mu.Unlock()

	activityCounter, found := m.activityCounters[key]
	if !found {
		activityCounter = NewActivityCounter()
		m.activityCounters[key] = activityCounter
	}

	return activityCounter
}

func (m *ActivityCounterMonitor) GetActivityCounter(key string) *ActivityCounter {
	m.mu.Lock()
	defer m.mu.Unlock()

	activityCounter, found := m.activityCounters[key]

	if !found {
		return nil
	}

	return activityCounter
}

func (m *ActivityCounterMonitor) ForEach(fn func(key string, ac *ActivityCounter)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, activityCounter := range m.activityCounters {
		fn(key, activityCounter)
	}
}
