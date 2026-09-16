package casbinx

import (
	"errors"
	"fmt"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// CasbinEnforcer 封装共享 Enforcer 与事务态 Enforcer 的切换逻辑。
type CasbinEnforcer struct {
	*casbin.Enforcer
	errInit     error
	model       model.Model
	tx          *gorm.DB
	policyState *sharedPolicyState
	// txEnforcerCache 缓存同一事务内构建的 Enforcer，避免重复创建。
	txEnforcerCache map[*gorm.DB]*casbin.Enforcer
}

// sharedPolicyState 在策略重新加载期间保护内存快照。
// 最新一次加载失败后拒绝继续使用旧快照，避免过期的允许规则继续授权。
type sharedPolicyState struct {
	mu        sync.RWMutex
	reloadErr error
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
	return enforcer.reloadPolicy()
}

func (e *CasbinEnforcer) reloadPolicy() error {
	if e.policyState == nil {
		e.policyState = &sharedPolicyState{}
	}
	e.policyState.mu.Lock()
	defer e.policyState.mu.Unlock()

	err := e.Enforcer.LoadPolicy()
	e.policyState.reloadErr = err
	return err
}

// Enforce 串行化策略读取与重载；最新一次重载失败时按 fail-closed 拒绝请求。
func (e *CasbinEnforcer) Enforce(rvals ...interface{}) (bool, error) {
	if e.policyState == nil {
		return e.Enforcer.Enforce(rvals...)
	}
	e.policyState.mu.RLock()
	defer e.policyState.mu.RUnlock()
	if e.policyState.reloadErr != nil {
		return false, fmt.Errorf("casbin policy unavailable after reload failure: %w", e.policyState.reloadErr)
	}
	return e.Enforcer.Enforce(rvals...)
}

// SetDB 返回一个绑定到指定事务的新 CasbinEnforcer。
func (e *CasbinEnforcer) SetDB(tx *gorm.DB) *CasbinEnforcer {
	return &CasbinEnforcer{
		Enforcer:    e.Enforcer,
		errInit:     e.errInit,
		model:       e.model,
		tx:          tx,
		policyState: e.policyState,
	}
}
