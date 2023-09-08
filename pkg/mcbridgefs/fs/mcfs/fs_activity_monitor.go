package mcfs

import (
	"sync"
	"sync/atomic"
	"time"
)

type PathBasedActivityCounterFactory struct {
	activityCounters map[string]*FSActivityCounter
	mu               sync.Mutex
}

func NewPathBasedActivityCounterFactory() *PathBasedActivityCounterFactory {
	return &PathBasedActivityCounterFactory{
		activityCounters: make(map[string]*FSActivityCounter),
	}
}

func (f *PathBasedActivityCounterFactory) GetOrCreateActivityCounter(path string) *FSActivityCounter {
	f.mu.Lock()
	defer f.mu.Unlock()

	activityCounter, found := f.activityCounters[path]
	if !found {
		activityCounter = NewFSActivityCounter()
		f.activityCounters[path] = activityCounter
	}

	return activityCounter
}

func (f *PathBasedActivityCounterFactory) GetActivityCounter(path string) *FSActivityCounter {
	f.mu.Lock()
	defer f.mu.Unlock()

	activityCounter, found := f.activityCounters[path]

	if !found {
		return nil
	}

	return activityCounter
}

func (f *PathBasedActivityCounterFactory) ForEach(fn func(ac *FSActivityCounter)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, activityCounter := range f.activityCounters {
		fn(activityCounter)
	}
}

type ActivityCounter interface {
	IncrementActivityCount()
}

type FSActivityCounter struct {
	activityCount         int64
	LastSeenActivityCount int64
	LastChanged           time.Time
}

func NewFSActivityCounter() *FSActivityCounter {
	return &FSActivityCounter{
		LastChanged: time.Now(),
	}
}

func (c *FSActivityCounter) IncrementActivityCount() {
	atomic.AddInt64(&(c.activityCount), 1)
}
