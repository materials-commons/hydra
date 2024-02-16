package config

var configer Configer = &DotenvConfig{}

func SetConfig(c Configer) {
	configer = c
}

func GetConfig() Configer {
	return configer
}

func LoadFromPath(path string) error {
	return configer.LoadFromPath(path)
}

func Load() error {
	return configer.Load()
}

func GetKey(key string) string {
	return configer.GetKey(key)
}

func MustGetKey(key string) string {
	return configer.MustGetKey(key)
}

func GetKeyWithDefault(key, defaultValue string) string {
	return configer.GetKeyWithDefault(key, defaultValue)
}

func GetIntKey(key string) int {
	return configer.GetIntKey(key)
}

func MustGetIntKey(key string) int {
	return configer.MustGetIntKey(key)
}

func GetIntKeyWithDefault(key string, defaultValue int) int {
	return configer.GetIntKeyWithDefault(key, defaultValue)
}
