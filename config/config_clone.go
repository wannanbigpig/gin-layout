package config

import (
	"fmt"
	"io"
	"os"

	"github.com/wannanbigpig/gin-layout/config/autoload"
)

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
		Queue:     cloneQueueConfig(autoload.Queue),
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

func cloneQueueConfig(src autoload.QueueConfig) autoload.QueueConfig {
	cloned := src
	if src.Queues != nil {
		cloned.Queues = make(map[string]int, len(src.Queues))
		for key, value := range src.Queues {
			cloned.Queues[key] = value
		}
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
		if !fileInfo.IsDir() {
			return nil
		}
		return fmt.Errorf("配置文件目录存在同名的文件夹，无法创建配置文件")
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("初始化失败: %w", err)
	}

	source, err := os.Open(exampleConfig)
	if err != nil {
		return fmt.Errorf("创建配置文件失败，配置示例文件不存在: %w", err)
	}
	defer func(source *os.File) {
		_ = source.Close()
	}(source)

	dst, err := os.Create(config)
	if err != nil {
		return fmt.Errorf("生成配置文件失败: %w", err)
	}
	defer func(dst *os.File) {
		_ = dst.Close()
	}(dst)

	if _, err := io.Copy(dst, source); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}
