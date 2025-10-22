package mctus2

import "sync"

type UploadProgressCache struct {
	uploadProgress map[string]int64
	mu             sync.Mutex
}

func NewUploadProgressCache() *UploadProgressCache {
	return &UploadProgressCache{
		uploadProgress: make(map[string]int64),
	}
}

func (c *UploadProgressCache) GetUploadProgress(uuid string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	progress, ok := c.uploadProgress[uuid]
	if !ok {
		return 0
	}

	return progress
}

func (c *UploadProgressCache) SetUploadProgress(uuid string, progress int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.uploadProgress[uuid] = progress
}

func (c *UploadProgressCache) DeleteUploadProgress(uuid string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.uploadProgress, uuid)
}
