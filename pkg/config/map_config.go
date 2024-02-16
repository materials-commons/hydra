package config

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/apex/log"
)

type MapConfig struct {
	configValues sync.Map
}

func NewMapConfig(entries map[string]string) *MapConfig {
	c := &MapConfig{}

	for key, entry := range entries {
		c.configValues.Store(key, entry)
	}

	return c
}

func (c *MapConfig) LoadFromPath(_ string) error {
	return fmt.Errorf("LoadFromPath not supported for MapConfig")
}

func (c *MapConfig) Load() error {
	return nil
}

func (c *MapConfig) GetKey(key string) string {
	v, ok := c.configValues.Load(key)
	switch {
	case !ok:
		return ""

	case v == nil:
		return ""

	default:
		return v.(string)
	}
}

func (c *MapConfig) MustGetKey(key string) string {
	val := c.GetKey(key)
	if val == "" {
		log.Fatalf("No such required config key: '%s'", key)
	}

	return val
}

func (c *MapConfig) GetKeyWithDefault(key, defaultValue string) string {
	val := c.GetKey(key)
	if val == "" {
		return defaultValue
	}

	return val
}

func (c *MapConfig) GetIntKey(key string) int {
	val := c.GetKey(key)
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}

	return intVal
}

func (c *MapConfig) MustGetIntKey(key string) int {
	val := c.GetKey(key)
	intVal, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("Required config key either doesn't exist or isn't an int: '%s': %s", key, err)
	}

	return intVal
}

func (c *MapConfig) GetIntKeyWithDefault(key string, defaultValue int) int {
	val := c.GetKey(key)
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}

	return intVal
}
