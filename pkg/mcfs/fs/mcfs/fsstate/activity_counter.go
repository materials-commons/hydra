package fsstate

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
	lastSeenActivityCount atomic.Uint64
	lastChangedAt         time.Time
	writesNotAllowed      atomic.Bool
	wantedWrite           sync.Map

	// Protects LastChanged
	mu sync.RWMutex
}

// NewActivityCounter creates a new ActivityCounter with LastChanged set to the current time.
func NewActivityCounter() *ActivityCounter {
	return &ActivityCounter{
		lastChangedAt: time.Now(),
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

func (c *ActivityCounter) PreventWrites() {
	c.writesNotAllowed.Store(true)
}

func (c *ActivityCounter) AllowWrites() {
	c.writesNotAllowed.Store(false)
}

func (c *ActivityCounter) WritesNotAllowed() bool {
	return c.writesNotAllowed.Load()
}

func (c *ActivityCounter) AddToWantedWrite(path string) {
	c.wantedWrite.Store(path, true)
}

func (c *ActivityCounter) ForEachWantedWrite(fn func(path string) bool) {
	c.wantedWrite.Range(func(key, value any) bool {
		shouldContinue := fn(key.(string))
		return shouldContinue
	})
}

func (c *ActivityCounter) ClearWantedWrite() {
	c.ForEachWantedWrite(func(path string) bool {
		c.wantedWrite.Delete(path)
		return true
	})
}

func (c *ActivityCounter) GetLastChangedAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastChangedAt
}

func (c *ActivityCounter) SetLastChangedAt(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastChangedAt = t
}
