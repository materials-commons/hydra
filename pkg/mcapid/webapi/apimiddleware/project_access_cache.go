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
		// Project is already known, let's check if the user is already in the cache and attempt
		// to load if they aren't. The checkAndLoadUser handles release the lock and will upgrade
		// to a write lock if needed.
		return c.checkAndLoadUser(userID, projectID, users)
	}

	// If we are here, then we need to load the project and check this user

	// 1. First upgrade to a write lock
	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	// 2. Check if the user has access
	if c.projectStor.UserCanAccessProject(userID, projectID) {
		// User has access. Since we haven't seen this project before, we need to add it
		// to the cache along with the user.
		c.cache[projectID] = []int{userID}
		return true, nil
	}

	// If we are here, then the user does not have access to the project. There is also
	// no entry for the project in the cache. We'll add an empty list to the cache.
	c.cache[projectID] = []int{}
	return false, nil
}

// checkAndLoadUser checks if the user is in the list of users for the project. If not,
// it will attempt to load the project and add the user to the list of users. It's
// important that this function is called with a read lock held. It will upgrade
// to a write lock if needed.
func (c *ProjectAccessCache) checkAndLoadUser(userID, projectID int, users []int) (bool, error) {
	// Check if the user is already in the list.
	for id, _ := range users {
		if id == userID {
			// We are holding a read lock, so we release and return.
			c.mu.RUnlock()
			return true, nil
		}
	}

	// User is not in the list. Before we upgrade the lock, we need to check if the user
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
