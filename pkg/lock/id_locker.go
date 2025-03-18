package lock

import (
	"sync"

	"github.com/apex/log"
)

type IdLocker struct {
	mapMutex sync.Mutex
	idMap    map[int]*sync.Mutex
}

func NewIdLocker() *IdLocker {
	return &IdLocker{
		idMap: make(map[int]*sync.Mutex),
	}
}

func (l *IdLocker) AcquireLock(id int) {
	l.mapMutex.Lock()
	defer l.mapMutex.Unlock()
	var m sync.Mutex
	idMutex, ok := l.idMap[id]
	if !ok {
		idMutex = &m
		l.idMap[id] = idMutex
	}
	idMutex.Lock()
}

func (l *IdLocker) ReleaseLock(id int) {
	m, ok := l.idMap[id]
	if !ok {
		log.Errorf("ReleaseLock called on id (%d) with no mutex", id)

		return
	}

	m.Unlock()
}

func (l *IdLocker) WithLock(id int, f func() error) error {
	l.AcquireLock(id)
	defer l.ReleaseLock(id)
	return f()
}
