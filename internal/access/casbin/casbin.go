package casbinx

import (
	"errors"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// CasbinEnforcer 封装共享 Enforcer 与事务态 Enforcer 的切换逻辑。
type CasbinEnforcer struct {
	*casbin.Enforcer
	errInit error
	model   model.Model
	tx      *gorm.DB
}

var (
	casbinManager = &CasbinEnforcer{}
	managerMu     sync.RWMutex
)

// InitEnforcer 初始化 Casbin Enforcer（仅执行一次）
func InitEnforcer() error {
	managerMu.Lock()
	defer managerMu.Unlock()
	if casbinManager.Enforcer != nil {
		return casbinManager.errInit
	}
	return initEnforcerLocked()
}

// GetEnforcer 返回已初始化的 Enforcer 实例
func GetEnforcer() (*CasbinEnforcer, error) {
	managerMu.RLock()
	current := casbinManager
	managerMu.RUnlock()
	if current.Enforcer == nil {
		if err := InitEnforcer(); err != nil {
			return nil, err
		}
	}
	managerMu.RLock()
	defer managerMu.RUnlock()
	if casbinManager.Enforcer == nil {
		return nil, errors.New("casbin enforcer not initialized")
	}
	return casbinManager, nil
}

// ReloadPolicy 重新加载策略
func ReloadPolicy() error {
	enforcer, err := GetEnforcer()
	if err != nil {
		return err
	}
	return enforcer.LoadPolicy()
}

// SetDB 返回一个绑定到指定事务的新 CasbinEnforcer。
func (e *CasbinEnforcer) SetDB(tx *gorm.DB) *CasbinEnforcer {
	return &CasbinEnforcer{
		Enforcer: e.Enforcer,
		errInit:  e.errInit,
		model:    e.model,
		tx:       tx,
	}
}

// RegisterCustomFunctions 注册自定义函数
func (e *CasbinEnforcer) registerCustomFunctions() {
	// 注册自定义函数
}
