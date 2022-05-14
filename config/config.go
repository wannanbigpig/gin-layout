package config

import (
	. "github.com/wannanbigpig/gin-layout/config/autoload"
	"gopkg.in/ini.v1"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Conf 配置项主结构体
type Conf struct {
	AppEnv         string        `ini:"app_env"`
	Language       string        `ini:"language"`
	StaticBasePath string        `ini:"base_path"`
	Server         *ServerConfig `ini:"server"`
	Mysql          *MysqlConfig  `ini:"mysql"`
	Logger         *LoggerConfig `ini:"logger"`
}

var (
	Once   sync.Once
	Config = Conf{
		AppEnv:         "local",
		Language:       "zh_CN",
		StaticBasePath: getDefaultBasePath(),
		Server:         Server,
		Mysql:          Mysql,
		Logger:         Logger,
	}
)

func init() {
	Once.Do(func() {
		load()
	})
}

// load 加载配置项
func load() {
	// 生成 config.ini 文件
	copyIniConf()
	cfg, err := ini.Load("config/config.ini")
	if err != nil {
		panic("读取配置文件失败: " + err.Error())
	}
	err = cfg.Section("app").MapTo(&Config)
	if err != nil {
		panic("加载配置失败: " + err.Error())
	}
}

// getDefaultUploadPath 获取静态文件存放路径
func getDefaultBasePath() string {
	currentPath, err := os.Getwd()
	if err != nil {
		panic("获取运行目录失败：" + err.Error())
	}
	return filepath.Join(currentPath, "/gin-layout")
}

// copyIniConf 复制config.ini文件
func copyIniConf() {
	iniConfig := "config/config.ini"
	iniExampleConfig := "config/config.example.ini"

	fileInfo, err := os.Stat(iniConfig)

	if err == nil {
		// config.ini 路径存在， 判断 config.ini 文件是否目录，存在则直接 return
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
	source, err := os.Open(iniExampleConfig)
	if err != nil {
		panic("创建配置文件失败，config.example.ini文件不存在: " + err.Error())
	}
	defer func(source *os.File) {
		err := source.Close()
		if err != nil {
			panic("关闭示例资源失败: " + err.Error())
		}
	}(source)

	// 创建空文件
	dst, err := os.Create(iniConfig)
	if err != nil {
		panic("生成config.ini失败: " + err.Error())
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
		panic("写入config.ini失败: " + err.Error())
	}
}
