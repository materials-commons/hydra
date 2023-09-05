package mcfs

import (
	"sync/atomic"
	"time"
)

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
