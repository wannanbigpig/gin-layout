package casbinx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	casbinmodel "github.com/casbin/casbin/v3/model"
	"gorm.io/gorm"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
)

// getModelPath 获取 rbac_model.conf 路径并校验是否存在
func getModelPath() (string, error) {
	const filename = "rbac_model.conf"
	candidates := []string{}
	if basePath := strings.TrimSpace(c.GetConfig().BasePath); basePath != "" {
		candidates = append(candidates, filepath.Join(basePath, filename))
	}
	if c.V != nil {
		if configPath := strings.TrimSpace(c.V.ConfigFileUsed()); configPath != "" {
			candidates = append(candidates, filepath.Join(filepath.Dir(configPath), filename))
		}
	}
	if workDir, err := os.Getwd(); err == nil && strings.TrimSpace(workDir) != "" {
		candidates = append(candidates, filepath.Join(workDir, filename))
	}
	if executablePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executablePath), filename))
	}
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(currentFile), "..", "..", "..", filename))
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, ok := seen[absPath]; ok {
			continue
		}
		seen[absPath] = struct{}{}
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			return absPath, nil
		}
	}
	return "", fmt.Errorf("模型文件不存在: %s", strings.Join(candidates, ", "))
}

// isInTransaction 判断是否在事务中
func isInTransaction(db *gorm.DB) bool {
	return db != nil && db.Statement != nil && db.Statement.ConnPool != db.ConnPool
}

// ReloadEnforcer 重新加载 Casbin Enforcer。
func ReloadEnforcer() error {
	managerMu.Lock()
	defer managerMu.Unlock()
	return initEnforcerLocked()
}

func initEnforcerLocked() error {
	modelPath, err := getModelPath()
	if err != nil {
		casbinManager.errInit = err
		return err
	}

	m, err := casbinmodel.NewModelFromFile(modelPath)
	if err != nil {
		casbinManager.errInit = fmt.Errorf("加载模型失败: %w", err)
		return casbinManager.errInit
	}

	db := data.MysqlDB()
	if db == nil {
		casbinManager.errInit = errors.New("mysql not initialized")
		return casbinManager.errInit
	}

	enforcer, err := newEnforcerFromDB(m, db)
	if err != nil {
		casbinManager.errInit = fmt.Errorf("创建 Enforcer 失败: %w", err)
		return casbinManager.errInit
	}
	next := &CasbinEnforcer{
		Enforcer: enforcer,
		model:    m,
	}
	next.registerCustomFunctions()
	casbinManager = next
	return nil
}
