package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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

	configPath, err := testConfigPath(rootPath)
	if err != nil {
		return nil, err
	}

	// 1、初始化配置：仅使用测试专用配置，禁止回退业务 config.yaml
	if err := config.InitConfig(configPath); err != nil {
		return nil, fmt.Errorf("加载测试配置失败: %w", err)
	}
	cfg := config.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("测试配置加载成功但配置对象为空")
	}
	if !cfg.Mysql.Enable {
		return nil, fmt.Errorf("接口测试必须在测试配置中启用真实 MySQL（配置: %s）", configPath)
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
		return nil, fmt.Errorf("测试配置指向的 MySQL 不可用（配置: %s）: %w", configPath, status.Error)
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

// testConfigPath 解析接口测试专用配置路径，不与业务 config.yaml 混用。
//
// 解析顺序：
//  1. 环境变量 GOTEST_CONFIG（或 TEST_CONFIG）
//  2. 项目根目录 config.test.yaml
//
// 两者都不存在时失败，不回退业务配置。
func testConfigPath(projectRoot string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("GOTEST_CONFIG")); v != "" {
		return resolveConfigFile(projectRoot, v, "GOTEST_CONFIG")
	}
	if v := strings.TrimSpace(os.Getenv("TEST_CONFIG")); v != "" {
		return resolveConfigFile(projectRoot, v, "TEST_CONFIG")
	}

	candidate := filepath.Join(projectRoot, "config.test.yaml")
	fileInfo, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("缺少测试配置 %s：请复制 config/config.test.yaml.example 为 config.test.yaml 并指向专用测试库，或设置 GOTEST_CONFIG 指向测试配置文件（不要使用业务 config.yaml）", candidate)
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("测试配置路径是目录: %s", candidate)
	}
	return candidate, nil
}

// resolveConfigFile 解析环境变量指定的配置路径。
// 相对路径按项目根目录解析，避免 go test 以包目录为 cwd 时找不到文件。
func resolveConfigFile(projectRoot, path, source string) (string, error) {
	if !filepath.IsAbs(path) {
		// 先尝试相对项目根；再退回相对当前工作目录（兼容从仓库根设置绝对语义的相对路径）
		candidate := filepath.Join(projectRoot, path)
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		} else {
			path = filepath.Clean(path)
		}
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s 指向的配置不存在: %s（已按项目根与 cwd 解析）", source, path)
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("%s 指向的配置是目录: %s", source, path)
	}
	return path, nil
}

// projectRootPath 返回项目根目录路径。
func projectRootPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to resolve project root path")
	}
	return filepath.Dir(filepath.Dir(file)), nil
}
