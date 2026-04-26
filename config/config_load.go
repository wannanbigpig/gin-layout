package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// InitConfig 初始化配置系统并加载首个生效快照。
func InitConfig(configPath string) error {
	once.Do(func() {
		var loaded *Conf
		loaded, initErr = load(configPath)
		if initErr != nil {
			return
		}
		initErr = validateJWTSecretKey(loaded)
		if initErr != nil {
			return
		}
		setActiveConfig(loaded)
	})
	return initErr
}

func checkJwtSecretKey() error {
	return validateJWTSecretKey(GetConfig())
}

func validateJWTSecretKey(cfg *Conf) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	secret := strings.TrimSpace(cfg.Jwt.SecretKey)
	if secret == "" {
		return fmt.Errorf("jwt.secret_key is empty, please set a non-empty secret key")
	}

	isProd := strings.EqualFold(cfg.AppEnv, "prod") || strings.EqualFold(cfg.AppEnv, "production")
	if !isProd {
		return nil
	}

	weakSecrets := map[string]struct{}{
		"<your_secret_key>":    {},
		"your-secret-key-here": {},
		"default-secret-key":   {},
		"change-me":            {},
		"changeme":             {},
		"secret":               {},
		"123456":               {},
	}
	if _, ok := weakSecrets[strings.ToLower(secret)]; ok {
		return fmt.Errorf("jwt.secret_key uses a weak placeholder value in production")
	}
	if len(secret) < 16 {
		return fmt.Errorf("jwt.secret_key is too short in production, require at least 16 characters")
	}

	return nil
}

func load(configPath string) (*Conf, error) {
	filePath, err := resolveConfigPath(configPath)
	if err != nil {
		return nil, err
	}

	V = viper.New()
	V.SetConfigFile(filePath)
	if err := V.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("未找到配置: %w", err)
		}
		return nil, fmt.Errorf("读取配置出错: %w", err)
	}

	loaded := cloneDefaultConfig()
	if err := V.Unmarshal(loaded); err != nil {
		return nil, fmt.Errorf("映射配置出错: %w", err)
	}

	resolveEnvVars(loaded)
	ensureBasePathDefault(loaded, V.IsSet("app.base_path"))

	ensureCorsDefaults(loaded)
	registerConfigWatcherIfNeeded(loaded)
	return loaded, nil
}

func resolveConfigPath(configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	exampleConfig, targetConfig, err := resolveDefaultConfigFiles()
	if err != nil {
		return "", err
	}
	if err := copyConf(exampleConfig, targetConfig); err != nil {
		return "", err
	}
	return targetConfig, nil
}

func resolveDefaultConfigFiles() (string, string, error) {
	if os.Getenv("GO_ENV") == "development" {
		return resolveDevelopmentConfigFiles()
	}

	runDirectory, err := utils.GetCurrentPath()
	if err != nil {
		return "", "", fmt.Errorf("获取执行文件目录失败: %w", err)
	}
	return filepath.Join(runDirectory, "config.yaml.example"), filepath.Join(runDirectory, "config.yaml"), nil
}

func resolveDevelopmentConfigFiles() (string, string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("获取工作目录失败: %w", err)
	}

	exampleConfig := filepath.Join(workDir, "config", "config.yaml.example")
	if !fileExists(exampleConfig) {
		exampleConfig = filepath.Join(workDir, "config.yaml.example")
	}
	return exampleConfig, filepath.Join(workDir, "config.yaml"), nil
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func ensureBasePathDefault(cfg *Conf, basePathConfigured bool) {
	if cfg == nil {
		return
	}
	if basePathConfigured && strings.TrimSpace(cfg.BasePath) != "" {
		return
	}

	if os.Getenv("GO_ENV") == "development" {
		workDir, err := os.Getwd()
		if err == nil && strings.TrimSpace(workDir) != "" {
			cfg.BasePath = workDir
			return
		}
	}

	runDir, err := utils.GetCurrentPath()
	if err == nil && strings.TrimSpace(runDir) != "" {
		cfg.BasePath = runDir
		return
	}

	workDir, err := os.Getwd()
	if err == nil && strings.TrimSpace(workDir) != "" {
		cfg.BasePath = workDir
		return
	}
	cfg.BasePath = strings.TrimSpace(cfg.BasePath)
	if cfg.BasePath == "" {
		cfg.BasePath = "."
	}
}

func registerConfigWatcherIfNeeded(cfg *Conf) {
	if !cfg.WatchConfig {
		return
	}

	V.WatchConfig()
	V.OnConfigChange(func(in fsnotify.Event) {
		initErr = reloadConfigFromWatcher()
	})
}

func ensureCorsDefaults(cfg *Conf) {
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
	if cfg.CorsMaxAge == 0 {
		cfg.CorsMaxAge = 43200
	}
}

func reloadConfigFromWatcher() error {
	if err := V.ReadInConfig(); err != nil {
		return fmt.Errorf("重新读取配置出错: %w", err)
	}

	next := cloneDefaultConfig()
	if err := V.Unmarshal(next); err != nil {
		return fmt.Errorf("重新映射配置出错: %w", err)
	}
	resolveEnvVars(next)
	ensureBasePathDefault(next, V.IsSet("app.base_path"))
	ensureCorsDefaults(next)
	if err := validateJWTSecretKey(next); err != nil {
		return fmt.Errorf("JWT 配置校验失败: %w", err)
	}

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

func resolveEnvVars(cfg *Conf) {
	cfg.Mysql.Username = resolveEnvVar(cfg.Mysql.Username)
	cfg.Mysql.Password = resolveEnvVar(cfg.Mysql.Password)
	cfg.Mysql.Host = resolveEnvVar(cfg.Mysql.Host)
	cfg.Redis.Password = resolveEnvVar(cfg.Redis.Password)
	cfg.Redis.Host = resolveEnvVar(cfg.Redis.Host)
	cfg.Queue.Redis.Password = resolveEnvVar(cfg.Queue.Redis.Password)
	cfg.Queue.Redis.Host = resolveEnvVar(cfg.Queue.Redis.Host)
	cfg.Jwt.SecretKey = resolveEnvVar(cfg.Jwt.SecretKey)
}

func resolveEnvVar(val string) string {
	if !strings.HasPrefix(val, "${") || !strings.HasSuffix(val, "}") {
		return val
	}
	envKey := val[2 : len(val)-1]
	if envVal := os.Getenv(envKey); envVal != "" {
		return envVal
	}
	return val
}
