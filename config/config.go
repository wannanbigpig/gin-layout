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
	Mysql              autoload.MysqlConfig    `mapstructure:"mysql"`
	Redis              autoload.RedisConfig    `mapstructure:"redis"`
	Logger             autoload.LoggerConfig   `mapstructure:"logger"`
	Jwt                autoload.JwtConfig      `mapstructure:"jwt"`
	Security           autoload.SecurityConfig `mapstructure:"security"`
	Queue              autoload.QueueConfig    `mapstructure:"queue"`
}

var (
	Config = &Conf{
		AppConfig: cloneAppConfig(autoload.App),
		Mysql:     autoload.Mysql,
		Redis:     autoload.Redis,
		Logger:    autoload.Logger,
		Jwt:       autoload.Jwt,
		Security:  autoload.Security,
		Queue:     cloneQueueConfig(autoload.Queue),
	}
	once        sync.Once
	initErrMu   sync.RWMutex
	initErr     error
	V           *viper.Viper
	configValue atomic.Value

	reloadHandlersMu sync.RWMutex
	reloadMu         sync.Mutex
	reloadHandlers   []ConfigReloadHandler
)

func setInitErr(err error) {
	initErrMu.Lock()
	defer initErrMu.Unlock()
	initErr = err
}

func getInitErr() error {
	initErrMu.RLock()
	defer initErrMu.RUnlock()
	return initErr
}

// ConfigReloadHandler 在配置热更新时被调用。
type ConfigReloadHandler struct {
	Name     string
	Priority int
	// Prepare 只能构建候选资源；全部处理器成功后才调用 Commit。
	Prepare func(oldConfig, newConfig *Conf, diff ConfigDiff) (ConfigReloadChange, error)
	// Handle 保留给无资源切换的校验回调；资源处理器应使用 Prepare。
	Handle func(oldConfig, newConfig *Conf, diff ConfigDiff) error
}

// ConfigReloadChange 将可能失败的准备过程与不会失败的实例切换分离。
type ConfigReloadChange struct {
	Commit   func()
	Abort    func()
	Finalize func()
}

// ConfigDiff 描述配置变更摘要。
type ConfigDiff struct {
	LoggerChanged                  bool
	MysqlChanged                   bool
	RedisChanged                   bool
	JWTChanged                     bool
	JWTSecretChanged               bool
	CredentialEncryptionKeyChanged bool
	BaseURLChanged                 bool
	CORSChanged                    bool
	TrustedProxiesChanged          bool
	LightAppChanged                bool
	RestartRequiredFields          []string
	ChangedFields                  []string
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
