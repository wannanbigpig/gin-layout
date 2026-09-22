package filestorage

import (
	"context"
	"errors"
	"strings"
)

const (
	DriverLocal     = "local"
	DriverAliyunOSS = "aliyun_oss"
)

var defaultRegistry = newDefaultRegistry()

func DefaultRegistry() *Registry {
	return defaultRegistry
}

// RegisterDriver 为二次开发提供编译期扩展入口。供应商包可在进程启动前注册
// DriverDefinition；重复名称会返回错误，避免后注册实现静默覆盖已有驱动。
func RegisterDriver(definition DriverDefinition) error {
	return defaultRegistry.Register(definition)
}

func newDefaultRegistry() *Registry {
	registry := NewRegistry()
	mustRegisterDriver(registry, DriverDefinition{
		Name: DriverLocal,
		Build: func(_ context.Context, config Config) (Driver, error) {
			return NewLocalDriver(config.Local, config.Local.PublicBasePath, config.Local.PrivateBasePath), nil
		},
		Validate: func(config Config) error {
			if strings.TrimSpace(config.Local.PublicBasePath) == "" || strings.TrimSpace(config.Local.PrivateBasePath) == "" {
				return errors.New("本地存储目录未配置")
			}
			return nil
		},
		ResolveBucket: func(_ Config, isPublic bool) string {
			if isPublic {
				return "public"
			}
			return "private"
		},
	})
	mustRegisterDriver(registry, DriverDefinition{
		Name: DriverAliyunOSS,
		Build: func(_ context.Context, config Config) (Driver, error) {
			return NewAliyunOSSDriver(config.AliyunOSS), nil
		},
		Validate: func(config Config) error {
			aliyun := config.AliyunOSS
			switch {
			case strings.TrimSpace(aliyun.Endpoint) == "":
				return errors.New("阿里云 OSS Endpoint 不能为空")
			case strings.TrimSpace(aliyun.Region) == "":
				return errors.New("阿里云 OSS Region 不能为空")
			case strings.TrimSpace(aliyun.Bucket) == "":
				return errors.New("阿里云 OSS Bucket 不能为空")
			case strings.TrimSpace(aliyun.AccessKeyID) == "":
				return errors.New("阿里云 OSS AccessKey ID 不能为空")
			case strings.TrimSpace(aliyun.AccessKeySecret) == "":
				return errors.New("阿里云 OSS AccessKey Secret 不能为空")
			default:
				return nil
			}
		},
		ResolveBucket: func(config Config, _ bool) string {
			return strings.TrimSpace(config.AliyunOSS.Bucket)
		},
		Capabilities: DriverCapabilities{DirectUpload: true, MultipartUpload: true, ImageThumbnail: true},
		SensitiveFields: []SensitiveField{{
			Name:  "access_key_secret",
			Label: "存储密钥",
			Get: func(config Config) string {
				return config.AliyunOSS.AccessKeySecret
			},
			Set: func(config *Config, value string) {
				config.AliyunOSS.AccessKeySecret = value
			},
		}},
	})
	return registry
}

func mustRegisterDriver(registry *Registry, definition DriverDefinition) {
	if err := registry.Register(definition); err != nil {
		panic(err)
	}
}
