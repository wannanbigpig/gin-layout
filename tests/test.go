package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/internal/validator"
)

// SetupRouter 初始化测试用路由。
func SetupRouter() (*gin.Engine, error) {
	rootPath, err := projectRootPath()
	if err != nil {
		return nil, err
	}

	configPath, err := testConfigPath()
	if err != nil {
		return nil, err
	}

	// 1、初始化配置
	if err := config.InitConfig(configPath); err != nil {
		return nil, err
	}
	cfg := config.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("测试配置加载成功但配置对象为空")
	}
	if !cfg.Mysql.Enable {
		return nil, fmt.Errorf("接口测试必须在项目根 config.yaml 中启用真实 MySQL")
	}
	cfg.BasePath = rootPath
	cfg.Mysql.PrintSql = false
	cfg.RateLimitEnabled = false
	cfg.LoginRateLimitEnabled = false
	// 2、初始化zap日志
	if err := logger.InitLogger(); err != nil {
		return nil, err
	}
	// 初始化数据库
	if err := data.InitData(); err != nil {
		return nil, err
	}
	if status := data.MysqlRuntimeStatus(); !status.Ready {
		return nil, fmt.Errorf("项目根 config.yaml 指向的 MySQL 不可用: %w", status.Error)
	}
	// 初始化验证器
	if err := validator.InitValidatorTrans("zh"); err != nil {
		return nil, err
	}

	engine, err := routers.SetRouters()
	if err != nil {
		return nil, err
	}
	return engine, nil
}

// testConfigPath 返回接口测试唯一允许使用的项目根配置文件路径。
func testConfigPath() (string, error) {
	projectRoot, err := projectRootPath()
	if err != nil {
		return "", err
	}
	projectConfigPath := filepath.Join(projectRoot, "config.yaml")
	fileInfo, err := os.Stat(projectConfigPath)
	if err != nil {
		return "", fmt.Errorf("接口测试要求项目根目录存在 config.yaml: %w", err)
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("项目根目录 config.yaml 是目录，无法作为测试配置文件")
	}
	return projectConfigPath, nil
}

// projectRootPath 返回项目根目录路径。
func projectRootPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to resolve project root path")
	}
	return filepath.Dir(filepath.Dir(file)), nil
}
