package config

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/spf13/viper"
	"github.com/wannanbigpig/gin-layout/config/autoload"
)

// Conf 配置项主结构体
type Conf struct {
	autoload.AppConfig `mapstructure:"app"`
	Mysql              autoload.MysqlConfig  `mapstructure:"mysql"`
	Redis              autoload.RedisConfig  `mapstructure:"redis"`
	Logger             autoload.LoggerConfig `mapstructure:"logger"`
	Jwt                autoload.JwtConfig    `mapstructure:"jwt"`
	Queue              autoload.QueueConfig  `mapstructure:"queue"`
}

var (
	Config = &Conf{
		AppConfig: cloneAppConfig(autoload.App),
		Mysql:     autoload.Mysql,
		Redis:     autoload.Redis,
		Logger:    autoload.Logger,
		Jwt:       autoload.Jwt,
		Queue:     cloneQueueConfig(autoload.Queue),
	}
	once        sync.Once
	initErr     error
	V           *viper.Viper
	configValue atomic.Value

	reloadHandlersMu sync.RWMutex
	reloadHandlers   []ConfigReloadHandler
)

// ConfigReloadHandler 在配置热更新时被调用。
type ConfigReloadHandler struct {
	Name     string
	Priority int
	Handle   func(oldConfig, newConfig *Conf, diff ConfigDiff) error
}

// ConfigDiff 描述配置变更摘要。
type ConfigDiff struct {
	LoggerChanged         bool
	MysqlChanged          bool
	RedisChanged          bool
	JWTChanged            bool
	JWTSecretChanged      bool
	BaseURLChanged        bool
	CORSChanged           bool
	TrustedProxiesChanged bool
	LightAppChanged       bool
	RestartRequiredFields []string
	ChangedFields         []string
}

// GetConfig 返回当前生效的配置快照。
func GetConfig() *Conf {
	if cfg, ok := configValue.Load().(*Conf); ok && cfg != nil {
		return cfg
	}
	return Config
}

// RegisterConfigReloadHandler 注册配置热更新回调。
func RegisterConfigReloadHandler(handler ConfigReloadHandler) {
	if handler.Name == "" {
		return
	}

	reloadHandlersMu.Lock()
	defer reloadHandlersMu.Unlock()

	for i := range reloadHandlers {
		if reloadHandlers[i].Name == handler.Name {
			reloadHandlers[i] = handler
			sortConfigReloadHandlersLocked()
			return
		}
	}

	reloadHandlers = append(reloadHandlers, handler)
	sortConfigReloadHandlersLocked()
}

func sortConfigReloadHandlersLocked() {
	sort.SliceStable(reloadHandlers, func(i, j int) bool {
		if reloadHandlers[i].Priority == reloadHandlers[j].Priority {
			return reloadHandlers[i].Name < reloadHandlers[j].Name
		}
		return reloadHandlers[i].Priority < reloadHandlers[j].Priority
	})
}
