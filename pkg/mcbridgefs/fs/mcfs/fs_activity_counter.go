package mcfs

import (
	"sync"
	"sync/atomic"
	"time"
)

type ActivityCounter struct {
	activityCount         int64
	LastSeenActivityCount int64
	LastChanged           time.Time
}

func NewActivityCounter() *ActivityCounter {
	return &ActivityCounter{
		LastChanged: time.Now(),
	}
}

func (c *ActivityCounter) IncrementActivityCount() {
	atomic.AddInt64(&(c.activityCount), 1)
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

func (m *ActivityCounterMonitor) GetOrCreateActivityCounter(path string) *ActivityCounter {
	m.mu.Lock()
	defer m.mu.Unlock()

	activityCounter, found := m.activityCounters[path]
	if !found {
		activityCounter = NewActivityCounter()
		m.activityCounters[path] = activityCounter
	}

	return activityCounter
}

func (m *ActivityCounterMonitor) GetActivityCounter(path string) *ActivityCounter {
	m.mu.Lock()
	defer m.mu.Unlock()

	activityCounter, found := m.activityCounters[path]

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
