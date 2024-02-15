package fsstate

import (
	"sync"
)

type ExpiredActivityHandlerFN func(activityKey string)

type ActivityTracker struct {
	activityCounters sync.Map
	//inactivityDuration     time.Duration
	//expiredActivityHandler ExpiredActivityHandlerFN
}

func NewActivityTracker() *ActivityTracker {
	return &ActivityTracker{
		//inactivityDuration: inactivityDuration,
	}
}

//func (m *ActivityTracker) Start(ctx context.Context) {
//	for {
//		if canceled := m.loadAndCheckEachActivityCounter(ctx); canceled {
//			break
//		}
//
//		select {
//		case <-ctx.Done():
//			break
//		case <-time.After(20 * time.Second):
//		}
//	}
//}
//
//func (m *ActivityTracker) loadAndCheckEachActivityCounter(c context.Context) bool {
//	isDone := false
//	now := time.Now()
//
//	m.activityCounters.Range(func(key, value any) bool {
//		if ctx.IsDone(c) {
//			isDone = true
//			return false
//		}
//
//		ac, ok := value.(*ActivityCounter)
//		if !ok {
//			return true
//		}
//		currentActivityCount := ac.GetActivityCount()
//		lastSeenActivityCount := ac.GetLastSeenActivityCount()
//
//		switch {
//		case currentActivityCount == lastSeenActivityCount:
//			lastChanged := ac.GetLastChanged()
//			allowedInactive := lastChanged.Add(m.inactivityDuration)
//			if now.After(allowedInactive) {
//				m.expiredActivityHandler(key.(string))
//			}
//
//		default:
//			ac.SetLastChanged(now)
//			ac.SetLastSeenActivityCount(currentActivityCount)
//		}
//
//		return true
//	})
//
//	return isDone
//}

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

func (m *ActivityTracker) ForEach(fn func(key string, ac *ActivityCounter)) {
	m.activityCounters.Range(func(key, value any) bool {
		kstr := key.(string)
		ac := value.(*ActivityCounter)
		fn(kstr, ac)
		return true
	})
}
