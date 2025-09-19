package mctus2

import (
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type AccessCache struct {
	usersByAPIKey       map[string]*mcmodel.User
	userIDToProjectList map[int][]int
	mu                  sync.Mutex
}

func NewAccessCache() *AccessCache {
	return &AccessCache{
		usersByAPIKey:       make(map[string]*mcmodel.User),
		userIDToProjectList: make(map[int][]int),
	}
}

func (c *AccessCache) Lock() {
	c.mu.Lock()
}

func (c *AccessCache) Unlock() {
	c.mu.Unlock()
}
