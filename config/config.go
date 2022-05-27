package config

import (
	. "github.com/wannanbigpig/gin-layout/config/autoload"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Conf 配置项主结构体
type Conf struct {
	AppConfig `ini:"app" yaml:"app"`
	Server    ServerConfig `ini:"server" yaml:"server"`
	Mysql     MysqlConfig  `ini:"mysql" yaml:"mysql"`
	Redis     RedisConfig  `ini:"redis" yaml:"redis"`
	Logger    LoggerConfig `ini:"logger" yaml:"logger"`
}

var Config = &Conf{
	AppConfig: App,
	Server:    Server,
	Mysql:     Mysql,
	Redis:     Redis,
	Logger:    Logger,
}

func init() {
	// 加载 .yaml 配置
	loadYaml()
	//fmt.Println(Config)
	// 加载 .ini 配置
	// loadIni()
}

func loadYaml() {
	currentDirectory, ok := utils.GetFileDirectoryToCaller()
	if !ok {
		panic("Failed to load configuration: Failed to obtain the current file directory")
	}

	// 生成 config.yaml 文件
	yamlConfig := filepath.Dir(currentDirectory) + "/config.yaml"
	yamlExampleConfig := currentDirectory + "/config.example.yaml"
	copyConf(yamlExampleConfig, yamlConfig)
	cfg, err := ioutil.ReadFile(yamlConfig)
	if err != nil {
		panic("Failed to read configuration file:" + err.Error())
	}
	err = yaml.Unmarshal(cfg, &Config)
	if err != nil {
		panic("Failed to load configuration:" + err.Error())
	}
}

// load 加载配置项
func loadIni() {
	currentDirectory, ok := utils.GetFileDirectoryToCaller()
	if !ok {
		panic("Failed to load configuration: Failed to obtain the current file directory")
	}

	// 生成 config.ini 文件
	iniConfig := filepath.Dir(currentDirectory) + "/config.ini"
	iniExampleConfig := currentDirectory + "/config.example.ini"
	copyConf(iniExampleConfig, iniConfig)
	cfg, err := ini.Load(iniConfig)
	if err != nil {
		panic("Failed to read configuration file:" + err.Error())
	}
	err = cfg.Section("app").MapTo(&Config)
	if err != nil {
		panic("Failed to load configuration:" + err.Error())
	}
}

// copyConf 复制配置示例文件
func copyConf(exampleConfig, config string) {
	fileInfo, err := os.Stat(config)

	if err == nil {
		// config.ini 路径存在， 判断 config.ini 文件是否目录，不是目录则代表文件存在直接 return
		if !fileInfo.IsDir() {
			return
		}
		panic("配置文件目录存在同名的文件夹，无法创建配置文件")
	}

	// 打开文件失败，并且返回的错误不是文件未找到
	if !os.IsNotExist(err) {
		panic("初始化失败: " + err.Error())
	}

	// 自动复制一份config.ini
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
