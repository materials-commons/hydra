package mcapid

import (
	"sync"

	"github.com/apex/log"
)

var projectMapMutex sync.Mutex
var projectMutexes = make(map[int]*sync.Mutex)

func AcquireProjectMutex(projectID int) {
	projectMapMutex.Lock()
	defer projectMapMutex.Unlock()
	var p sync.Mutex
	projectMutex, ok := projectMutexes[projectID]
	if !ok {
		projectMutex = &p
		projectMutexes[projectID] = projectMutex
	}
	projectMutex.Lock()
}

func ReleaseProjectMutex(projectID int) {
	m, ok := projectMutexes[projectID]
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
