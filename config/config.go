package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	"github.com/wannanbigpig/gin-layout/config/autoload"
	utils2 "github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// Conf 配置项主结构体
type Conf struct {
	autoload.AppConfig `mapstructure:"app"`
	Mysql              autoload.MysqlConfig  `mapstructure:"mysql"`
	Redis              autoload.RedisConfig  `mapstructure:"redis"`
	Logger             autoload.LoggerConfig `mapstructure:"logger"`
	Jwt                autoload.JwtConfig    `mapstructure:"jwt"`
}

var (
	Config = &Conf{
		AppConfig: cloneAppConfig(autoload.App),
		Mysql:     autoload.Mysql,
		Redis:     autoload.Redis,
		Logger:    autoload.Logger,
		Jwt:       autoload.Jwt,
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

// InitConfig 初始化配置系统并加载首个生效快照。
func InitConfig(configPath string) error {
	once.Do(func() {
		// 加载 .yaml 配置
		var loaded *Conf
		loaded, initErr = load(configPath)
		if initErr != nil {
			return
		}
		setActiveConfig(loaded)

		// 检查jwtSecretKey
		initErr = checkJwtSecretKey()
	})
	return initErr
}

// checkJwtSecretKey 检查jwtSecretKey
func checkJwtSecretKey() error {
	if Config.Jwt.SecretKey == "" {
		Config.Jwt.SecretKey = utils2.RandString(64)
		configValue.Store(Config)
		fmt.Fprintf(
			os.Stderr,
			"warning: jwt.secret_key is empty, generated an in-memory secret for this process; set jwt.secret_key in config.yaml to persist it across restarts\n",
		)
	}
	return nil
}

func load(configPath string) (*Conf, error) {
	var filePath string
	if configPath == "" {
		// 判断是否为开发模式
		isDevelopment := os.Getenv("GO_ENV") == "development"

		var exampleConfig, targetConfig string

		if isDevelopment {
			// 开发模式：从当前工作目录查找配置文件
			workDir, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("获取工作目录失败: %w", err)
			}
			// 先尝试项目根目录下的 config/ 目录
			exampleConfig = filepath.Join(workDir, "config", "config.yaml.example")
			if _, err := os.Stat(exampleConfig); os.IsNotExist(err) {
				// 再尝试项目根目录
				exampleConfig = filepath.Join(workDir, "config.yaml.example")
			}
			targetConfig = filepath.Join(workDir, "config.yaml")
		} else {
			// 生产模式：从执行文件目录查找配置文件
			runDirectory, err := utils.GetCurrentPath()
			if err != nil {
				return nil, fmt.Errorf("获取执行文件目录失败: %w", err)
			}
			exampleConfig = filepath.Join(runDirectory, "config.yaml.example")
			targetConfig = filepath.Join(runDirectory, "config.yaml")
		}

		filePath = targetConfig
		if err := copyConf(exampleConfig, filePath); err != nil {
			return nil, err
		}
	} else {
		filePath = configPath
	}
	V = viper.New()
	// 路径必须要写相对路径,相对于项目的路径
	V.SetConfigFile(filePath)

	if err := V.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("未找到配置: %w", err)
		} else {
			return nil, fmt.Errorf("读取配置出错: %w", err)
		}
	}

	loaded := cloneDefaultConfig()
	// 映射到结构体
	if err := V.Unmarshal(loaded); err != nil {
		return nil, fmt.Errorf("映射配置出错: %w", err)
	}

	// 确保 CORS 配置字段有默认值（防止 nil 指针）
	ensureCorsDefaults(loaded)

	// 默认不监听配置变化，有些配置例如端口，数据库连接等即时配置变化不重启也不会变更。会导致配置文件与实际监听端口不一致混淆
	if loaded.WatchConfig {
		// 监听配置文件变化
		V.WatchConfig()
		V.OnConfigChange(func(in fsnotify.Event) {
			initErr = reloadConfigFromWatcher()
		})
	}
	return loaded, nil
}

// ensureCorsDefaults 确保 CORS 配置字段有默认值
func ensureCorsDefaults(cfg *Conf) {
	// 如果切片为 nil，初始化为空切片
	if cfg.CorsOrigins == nil {
		cfg.CorsOrigins = []string{}
	}
	if cfg.CorsMethods == nil {
		cfg.CorsMethods = []string{}
	}
	if cfg.CorsHeaders == nil {
		cfg.CorsHeaders = []string{}
	}
	if cfg.CorsExposeHeaders == nil {
		cfg.CorsExposeHeaders = []string{}
	}
	if cfg.TrustedProxies == nil {
		cfg.TrustedProxies = []string{"127.0.0.1"}
	}
	// CorsMaxAge 和 CorsCredentials 是基本类型，不需要检查 nil
	// 但如果为 0，使用默认值
	if cfg.CorsMaxAge == 0 {
		cfg.CorsMaxAge = 43200 // 默认 12 小时
	}
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

func reloadConfigFromWatcher() error {
	if err := V.ReadInConfig(); err != nil {
		return fmt.Errorf("重新读取配置出错: %w", err)
	}

	next := cloneDefaultConfig()
	if err := V.Unmarshal(next); err != nil {
		return fmt.Errorf("重新映射配置出错: %w", err)
	}
	ensureCorsDefaults(next)

	current := GetConfig()
	diff := BuildConfigDiff(current, next)
	applied := BuildAppliedConfig(current, next, diff)

	reloadHandlersMu.RLock()
	handlers := append([]ConfigReloadHandler(nil), reloadHandlers...)
	reloadHandlersMu.RUnlock()

	for _, handler := range handlers {
		if handler.Handle == nil {
			continue
		}
		if err := handler.Handle(current, applied, diff); err != nil {
			return fmt.Errorf("配置热更新失败[%s]: %w", handler.Name, err)
		}
	}

	setActiveConfig(applied)
	return nil
}

func setActiveConfig(cfg *Conf) {
	Config = cfg
	configValue.Store(cfg)
}

func cloneDefaultConfig() *Conf {
	return &Conf{
		AppConfig: cloneAppConfig(autoload.App),
		Mysql:     autoload.Mysql,
		Redis:     autoload.Redis,
		Logger:    autoload.Logger,
		Jwt:       autoload.Jwt,
	}
}

func cloneAppConfig(src autoload.AppConfig) autoload.AppConfig {
	cloned := src
	cloned.TrustedProxies = cloneStringSlice(src.TrustedProxies)
	cloned.CorsOrigins = cloneStringSlice(src.CorsOrigins)
	cloned.CorsMethods = cloneStringSlice(src.CorsMethods)
	cloned.CorsHeaders = cloneStringSlice(src.CorsHeaders)
	cloned.CorsExposeHeaders = cloneStringSlice(src.CorsExposeHeaders)
	if src.Timezone != nil {
		tz := *src.Timezone
		cloned.Timezone = &tz
	}
	return cloned
}

func cloneStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	return append([]string(nil), src...)
}

// copyConf 复制配置示例文件
func copyConf(exampleConfig, config string) error {
	fileInfo, err := os.Stat(config)

	if err == nil {
		// 路径存在， 判断 config 文件是否目录，不是目录则代表文件存在直接 return
		if !fileInfo.IsDir() {
			return nil
		}
		return fmt.Errorf("配置文件目录存在同名的文件夹，无法创建配置文件")
	}

	// 打开文件失败，并且返回的错误不是文件未找到
	if !os.IsNotExist(err) {
		return fmt.Errorf("初始化失败: %w", err)
	}

	// 自动复制一份示例文件
	source, err := os.Open(exampleConfig)
	if err != nil {
		return fmt.Errorf("创建配置文件失败，配置示例文件不存在: %w", err)
	}
	defer func(source *os.File) {
		_ = source.Close()
	}(source)

	// 创建空文件
	dst, err := os.Create(config)
	if err != nil {
		return fmt.Errorf("生成配置文件失败: %w", err)
	}
	defer func(dst *os.File) {
		_ = dst.Close()
	}(dst)

	// 复制内容
	_, err = io.Copy(dst, source)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}
