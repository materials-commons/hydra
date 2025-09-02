package apimiddleware

import (
	"sync"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

type APIKeyCache struct {
	apikeyCacheMu sync.RWMutex
	cache         map[string]*mcmodel.User
	userStor      stor.UserStor
}

func NewAPIKeyCache(userStor stor.UserStor) *APIKeyCache {
	return &APIKeyCache{
		cache:    make(map[string]*mcmodel.User),
		userStor: userStor,
	}
}

func (c *APIKeyCache) GetUserByAPIKey(apikey string) (*mcmodel.User, error) {
	c.apikeyCacheMu.RLock()

	if user, ok := c.cache[apikey]; ok {
		c.apikeyCacheMu.RUnlock()
		return user, nil
	}

	// Need to upgrade to a Write Lock
	c.apikeyCacheMu.RUnlock()
	c.apikeyCacheMu.Lock()
	defer c.apikeyCacheMu.Unlock()

	// Now that we've upgraded check again if the user exists. We do this
	// because a different thread may have acquired and created the user
	// in between us releasing the read lock and acquiring the write lock.
	if user, ok := c.cache[apikey]; ok {
		return user, nil
	}

	// User doesn't exist so retrieve from database, put into cache and return
	user, err := c.userStor.GetUserByAPIToken(apikey)
	if err != nil {
		// No user matching that key
		return nil, err
	}

	c.cache[apikey] = user
	return user, nil
}

func (c *APIKeyCache) DeleteUserByAPIKey(apikey string) {
	c.apikeyCacheMu.Lock()
	defer c.apikeyCacheMu.Unlock()
	delete(c.cache, apikey)
}
