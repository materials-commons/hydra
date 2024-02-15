package fsstate

import (
	"sync"
)

type ExpiredActivityHandlerFN func(activityKey string)

type ActivityTracker struct {
	activityCounters sync.Map
}

func NewActivityTracker() *ActivityTracker {
	return &ActivityTracker{}
}

func (m *ActivityTracker) GetOrCreateActivityCounter(key string) *ActivityCounter {
	activityCounter := NewActivityCounter()
	ac, _ := m.activityCounters.LoadOrStore(key, activityCounter)

	return ac.(*ActivityCounter)
}

func (m *ActivityTracker) GetActivityCounter(key string) *ActivityCounter {
	activityCounter, found := m.activityCounters.Load(key)
	if !found {
		return nil
	}

	return activityCounter.(*ActivityCounter)
}

func (m *ActivityTracker) ForEach(fn func(key string, ac *ActivityCounter) error) {
	m.activityCounters.Range(func(key, value any) bool {
		kstr := key.(string)
		ac := value.(*ActivityCounter)
		if err := fn(kstr, ac); err != nil {
			return false
		}
		return true
	})
}

func (m *ActivityTracker) RemoveActivityFromTracking(key string) {
	m.activityCounters.Delete(key)
}
