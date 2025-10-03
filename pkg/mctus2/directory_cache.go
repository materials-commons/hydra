package mctus2

import (
	"sync"
)

type DirectoryCache struct {
	projectIDToDirectoryID map[int][]int
	mu                     sync.Mutex
}

func NewDirectoryCache() *DirectoryCache {
	return &DirectoryCache{
		projectIDToDirectoryID: make(map[int][]int),
	}
}

func (c *DirectoryCache) Lock() {
	c.mu.Lock()
}

func (c *DirectoryCache) Unlock() {
	c.mu.Unlock()
}

func (c *DirectoryCache) DirectoryIDExists(projectID int, directoryIDToFind int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Does the project exist?
	directoryIDs, ok := c.projectIDToDirectoryID[projectID]
	if !ok {
		// Not found return false
		return false
	}

	// Found the project, does the directory exist?
	for _, directoryID := range directoryIDs {
		if directoryID == directoryIDToFind {
			return true
		}
	}

	// If we are here, then the directory wasn't found
	return false
}
