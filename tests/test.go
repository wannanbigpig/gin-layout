package tests

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

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
	if cfg != nil {
		cfg.BasePath = rootPath
		cfg.Mysql.PrintSql = false
	}
	// 2、初始化zap日志
	if err := logger.InitLogger(); err != nil {
		return nil, err
	}
	// 初始化数据库
	if err := data.InitData(); err != nil {
		return nil, err
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

// testConfigPath 返回测试运行使用的临时配置文件路径。
func testConfigPath() (string, error) {
	projectRoot, err := projectRootPath()
	if err != nil {
		return "", err
	}
	projectConfigPath := filepath.Join(projectRoot, "config.yaml")
	if fileInfo, err := os.Stat(projectConfigPath); err == nil {
		if fileInfo.IsDir() {
			return "", fmt.Errorf("项目根目录 config.yaml 是目录，无法作为测试配置文件")
		}
		if isProjectConfigUsable(projectConfigPath) {
			return projectConfigPath, nil
		}
	}

	examplePath := filepath.Join(projectRoot, "config", "config.yaml.example")
	content, err := os.ReadFile(examplePath)
	if err != nil {
		return "", err
	}
	tempPath := filepath.Join(os.TempDir(), "go-layout-test-config.yaml")
	if err := os.WriteFile(tempPath, content, 0o600); err != nil {
		return "", err
	}
	return tempPath, nil
}

// projectRootPath 返回项目根目录路径。
func projectRootPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to resolve project root path")
	}
	return filepath.Dir(filepath.Dir(file)), nil
}

// isProjectConfigUsable 判断根目录配置是否适合当前测试环境直接使用。
func isProjectConfigUsable(configPath string) bool {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(content)); err != nil {
		return false
	}

	if v.GetBool("mysql.enable") && !canDial(v.GetString("mysql.host"), v.GetInt("mysql.port")) {
		return false
	}
	if v.GetBool("redis.enable") && !canDial(v.GetString("redis.host"), v.GetInt("redis.port")) {
		return false
	}

	return true
}

// canDial 检查测试环境是否能连接到指定地址。
func canDial(host string, port int) bool {
	if host == "" || port == 0 {
		return false
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
