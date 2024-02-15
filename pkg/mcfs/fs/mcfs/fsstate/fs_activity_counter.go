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
