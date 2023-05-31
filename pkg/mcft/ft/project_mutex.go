package ft

import (
	"sync"

	"github.com/apex/log"
)

var mapMutex sync.Mutex
var mutexes = make(map[int]*sync.Mutex)

func acquireProjectMutex(projectID int) {
	mapMutex.Lock()
	defer mapMutex.Unlock()
	var p sync.Mutex
	projectMutex, ok := mutexes[projectID]
	if !ok {
		projectMutex = &p
		mutexes[projectID] = projectMutex
	}
	projectMutex.Lock()
}

func releaseProjectMutex(projectID int) {
	m, ok := mutexes[projectID]
	if !ok {
		log.Errorf("releaseProjectMutex called on project (%d) with no mutex", projectID)
		return
	}

	m.Unlock()
}

// ensureProjectMutexReleased will make sure that the project mutex
// is unlocked. Because it is unknown when this will be called and
// what the state of the mutex is, it will attempt to acquire it
// and then release it. This is done because Unlock() cannot be
// called on a Mutex that hasn't been locked.
func ensureProjectMutexReleased(projectID int) {
	mapMutex.Lock()
	defer mapMutex.Unlock()
	m, ok := mutexes[projectID]
	if !ok {
		return
	}

	m.TryLock()
	m.Unlock()
}
