package autoload

import (
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

type AppConfig struct {
	AppEnv         string  `mapstructure:"app_env"`
	Debug          bool    `mapstructure:"debug"`
	Language       string  `mapstructure:"language"`
	WatchConfig    bool    `mapstructure:"watch_config"`
	StaticBasePath string  `mapstructure:"base_path"`
	Timezone       *string `mapstructure:"timezone"`
}

var App = AppConfig{
	AppEnv:         "local",
	Debug:          true,
	Language:       "zh_CN",
	WatchConfig:    false,
	StaticBasePath: getDefaultPath(),
	Timezone:       nil,
}

func getDefaultPath() (path string) {
	path, _ = utils.GetDefaultPath()
	path = utils.If(path != "", path, "/tmp")
	return
}
