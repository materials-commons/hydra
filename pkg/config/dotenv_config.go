package config

import (
	"os"
	"strconv"

	"github.com/apex/log"
	"github.com/subosito/gotenv"
)

type DotenvConfig struct {
	DotenvPath string
}

func NewDotenvConfig(path string) *DotenvConfig {
	return &DotenvConfig{DotenvPath: path}
}

func (c *DotenvConfig) LoadFromPath(path string) error {
	c.DotenvPath = path
	return gotenv.Load(c.DotenvPath)
}

func (c *DotenvConfig) Load() error {
	return gotenv.Load(c.DotenvPath)
}

func (c *DotenvConfig) GetKey(key string) string {
	return os.Getenv(key)
}

func (c *DotenvConfig) MustGetKey(key string) string {
	val := c.GetKey(key)
	if val == "" {
		log.Fatalf("No such required config key: '%s'", key)
	}

	return val
}

func (c *DotenvConfig) GetKeyWithDefault(key, defaultValue string) string {
	val := c.GetKey(key)
	if val == "" {
		return defaultValue
	}

	return val
}

func (c *DotenvConfig) GetIntKey(key string) int {
	val := c.GetKey(key)
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}

	return intVal
}

func (c *DotenvConfig) MustGetIntKey(key string) int {
	val := c.GetKey(key)
	intVal, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("Required config key either doesn't exist or isn't an int: '%s': %s", key, err)
	}

	return intVal
}

func (c *DotenvConfig) GetIntKeyWithDefault(key string, defaultValue int) int {
	val := c.GetKey(key)
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}

	return intVal
}
