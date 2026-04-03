package casbinx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
)

// getModelPath 获取 rbac_model.conf 路径并校验是否存在
func getModelPath() (string, error) {
	path := filepath.Join(c.GetConfig().BasePath, "rbac_model.conf")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("模型文件不存在: %s", path)
	}
	return path, nil
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

	m, err := model.NewModelFromFile(modelPath)
	if err != nil {
		casbinManager.errInit = fmt.Errorf("加载模型失败: %w", err)
		return casbinManager.errInit
	}

	db := data.MysqlDB()
	if db == nil {
		casbinManager.errInit = errors.New("mysql not initialized")
		return casbinManager.errInit
	}

	gormadapter.TurnOffAutoMigrate(db)
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		casbinManager.errInit = fmt.Errorf("创建适配器失败: %w", err)
		return casbinManager.errInit
	}

	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		casbinManager.errInit = fmt.Errorf("创建 Enforcer 失败: %w", err)
		return casbinManager.errInit
	}

	enforcer.EnableAutoSave(true)
	next := &CasbinEnforcer{
		Enforcer: enforcer,
		model:    m,
	}
	next.registerCustomFunctions()
	casbinManager = next
	return nil
}
