package apimiddleware

import (
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type ProjectAccessCache struct {
	mu          sync.RWMutex
	cache       map[int][]int // map project id to a list of user ids that can access the project
	projectStor stor.ProjectStor
}

func NewProjectAccessCache(projectStor stor.ProjectStor) *ProjectAccessCache {
	return &ProjectAccessCache{
		cache:       make(map[int][]int),
		projectStor: projectStor,
	}
}

func (c *ProjectAccessCache) HasAccessToProject(userID, projectID int) (bool, error) {
	c.mu.RLock()

	users, ok := c.cache[projectID]
	if ok {
		// Project is already known, lets check if user is already in the cache and attempt
		// to load if they aren't. The checkAndLoadUser will upgrade to a write lock if needed.
		return c.checkAndLoadUser(userID, projectID, users)
	}

	// If we are here then we need to load the project and check this user

	// 1. First upgrade to a write lock
	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if user has access
	if c.projectStor.UserCanAccessProject(userID, projectID) {
		// User has access, let's add them to the cache. Since we haven't seen
		// this project before we can just add it to the cache.
		c.cache[projectID] = []int{userID}
		return true, nil
	}

	// If we are here, then the user does not have access to the project. Let's
	// create the project entry and an empty array of users for it.
	c.cache[projectID] = []int{}
	return false, nil
}

// checkAndLoadUser checks if the user is in the list of users for the project. If not,
// it will attempt to load the project and add the user to the list of users.
func (c *ProjectAccessCache) checkAndLoadUser(userID, projectID int, users []int) (bool, error) {
	// Check if user is already in the list
	for id, _ := range users {
		if id == userID {
			// We are holding a read lock, the parent function doesn't know what lock
			// we are holding, so we release and return.
			c.mu.RUnlock()
			return true, nil
		}
	}

	// User is not in the list. Before we upgrade we need to check if the user
	// even has access to the project.
	if !c.projectStor.UserCanAccessProject(userID, projectID) {
		// No access, so release the read lock and return false
		c.mu.RUnlock()
		return false, nil
	}

	// User access is ok, so upgrade to a write lock and add the user to the list
	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add user to the list and update the map
	users = append(users, userID)
	c.cache[projectID] = users

	// The defer above will release the write lock, so we can return true
	return true, nil
}
