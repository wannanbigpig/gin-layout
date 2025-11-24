package config

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	. "github.com/wannanbigpig/gin-layout/config/autoload"
	utils2 "github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// Conf 配置项主结构体
type Conf struct {
	AppConfig `mapstructure:"app"`
	Mysql     MysqlConfig  `mapstructure:"mysql"`
	Redis     RedisConfig  `mapstructure:"redis"`
	Logger    LoggerConfig `mapstructure:"logger"`
	Jwt       JwtConfig    `mapstructure:"jwt"`
}

var (
	Config = &Conf{
		AppConfig: App,
		Mysql:     Mysql,
		Redis:     Redis,
		Logger:    Logger,
		Jwt:       Jwt,
	}
	once sync.Once
	V    *viper.Viper
)

func InitConfig(configPath string) {
	once.Do(func() {
		// 加载 .yaml 配置
		load(configPath)

		// 检查jwtSecretKey
		checkJwtSecretKey()
	})
}

// checkJwtSecretKey 检查jwtSecretKey
func checkJwtSecretKey() {
	// 自动生成JWT secretKey
	if Config.Jwt.SecretKey == "" {
		Config.Jwt.SecretKey = utils2.RandString(64)
		V.Set("jwt.secret_key", Config.Jwt.SecretKey)
		err := V.WriteConfig()
		if err != nil {
			panic("自动生成JWT secretKey失败: " + err.Error())
		}
	}
}

func load(configPath string) {
	var filePath string
	if configPath == "" {
		// 判断是否为开发模式
		isDevelopment := os.Getenv("GO_ENV") == "development"

		var exampleConfig, targetConfig string

		if isDevelopment {
			// 开发模式：从当前工作目录查找配置文件
			workDir, err := os.Getwd()
			if err != nil {
				panic("获取工作目录失败: " + err.Error())
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
				panic("获取执行文件目录失败: " + err.Error())
			}
			exampleConfig = filepath.Join(runDirectory, "config.yaml.example")
			targetConfig = filepath.Join(runDirectory, "config.yaml")
		}

		filePath = targetConfig
		copyConf(exampleConfig, filePath)
	} else {
		filePath = configPath
	}
	V = viper.New()
	// 路径必须要写相对路径,相对于项目的路径
	V.SetConfigFile(filePath)

	if err := V.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			panic("未找到配置: " + err.Error())
		} else {
			panic("读取配置出错：" + err.Error())
		}
	}

	// 映射到结构体
	if err := V.Unmarshal(&Config); err != nil {
		panic(err)
	}

	// 确保 CORS 配置字段有默认值（防止 nil 指针）
	ensureCorsDefaults()

	// 默认不监听配置变化，有些配置例如端口，数据库连接等即时配置变化不重启也不会变更。会导致配置文件与实际监听端口不一致混淆
	if Config.WatchConfig {
		// 监听配置文件变化
		V.WatchConfig()
		V.OnConfigChange(func(in fsnotify.Event) {
			if err := V.ReadInConfig(); err != nil {
				panic(err)
			}
			if err := V.Unmarshal(&Config); err != nil {
				panic(err)
			}
			// 确保 CORS 配置字段有默认值
			ensureCorsDefaults()
		})
	}
}

// ensureCorsDefaults 确保 CORS 配置字段有默认值
func ensureCorsDefaults() {
	// 如果切片为 nil，初始化为空切片
	if Config.CorsOrigins == nil {
		Config.CorsOrigins = []string{}
	}
	if Config.CorsMethods == nil {
		Config.CorsMethods = []string{}
	}
	if Config.CorsHeaders == nil {
		Config.CorsHeaders = []string{}
	}
	if Config.CorsExposeHeaders == nil {
		Config.CorsExposeHeaders = []string{}
	}
	// CorsMaxAge 和 CorsCredentials 是基本类型，不需要检查 nil
	// 但如果为 0，使用默认值
	if Config.CorsMaxAge == 0 {
		Config.CorsMaxAge = 43200 // 默认 12 小时
	}
}

// copyConf 复制配置示例文件
func copyConf(exampleConfig, config string) {
	fileInfo, err := os.Stat(config)

	if err == nil {
		// 路径存在， 判断 config 文件是否目录，不是目录则代表文件存在直接 return
		if !fileInfo.IsDir() {
			return
		}
		panic("配置文件目录存在同名的文件夹，无法创建配置文件")
	}

	// 打开文件失败，并且返回的错误不是文件未找到
	if !os.IsNotExist(err) {
		panic("初始化失败: " + err.Error())
	}

	// 自动复制一份示例文件
	source, err := os.Open(exampleConfig)
	if err != nil {
		panic("创建配置文件失败，配置示例文件不存在: " + err.Error())
	}
	defer func(source *os.File) {
		err := source.Close()
		if err != nil {
			panic("关闭示例资源失败: " + err.Error())
		}
	}(source)

	// 创建空文件
	dst, err := os.Create(config)
	if err != nil {
		panic("生成配置文件失败: " + err.Error())
	}
	defer func(dst *os.File) {
		err := dst.Close()
		if err != nil {
			panic("关闭资源失败: " + err.Error())
		}
	}(dst)

	// 复制内容
	_, err = io.Copy(dst, source)
	if err != nil {
		panic("写入配置文件失败: " + err.Error())
	}
}
