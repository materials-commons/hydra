package config

type Configer interface {
	LoadFromPath(path string) error
	Load() error
	GetKey(key string) string
	MustGetKey(key string) string
	GetKeyWithDefault(key, defaultValue string) string
	GetIntKey(key string) int
	MustGetIntKey(key string) int
	GetIntKeyWithDefault(key string, defaultValue int) int
}
