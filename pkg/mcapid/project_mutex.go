package mcapid

import (
	"sync"

	"github.com/apex/log"
)

var mapMutex sync.Mutex
var mutexes = make(map[int]*sync.Mutex)

func AcquireProjectMutex(projectID int) {
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

func ReleaseProjectMutex(projectID int) {
	m, ok := mutexes[projectID]
	if !ok {
		log.Errorf("releaseProjectMutex called on project (%d) with no mutex", projectID)
		return
	}

	m.Unlock()
}

func WithProjectMutex(projectID int, f func() error) error {
	AcquireProjectMutex(projectID)
	defer ReleaseProjectMutex(projectID)
	return f()
}
