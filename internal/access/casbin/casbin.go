package casbinx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// CasbinEnforcer 封装共享 Enforcer 与事务态 Enforcer 的切换逻辑。
type CasbinEnforcer struct {
	*casbin.Enforcer
	errInit      error
	model        model.Model
	tx           *gorm.DB
	policyState  *sharedPolicyState
	versionStore policyVersionAccess
	// txEnforcerCache 缓存同一事务内构建的 Enforcer，避免重复创建。
	txEnforcerCache map[*gorm.DB]*casbin.Enforcer
}

// sharedPolicyState 在策略重新加载期间保护内存快照。
// 最新一次加载失败后拒绝继续使用旧快照，避免过期的允许规则继续授权。
type sharedPolicyState struct {
	mu                sync.RWMutex
	reloadErr         error
	lastReloadAttempt time.Time
	loadedVersion     uint64
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
	// 调用方在权限事务提交后刷新；本机加载失败也必须通知其他副本。
	defer notifyPolicyChanged()
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

	err := e.reloadPolicyLocked()
	e.policyState.reloadErr = err
	e.policyState.lastReloadAttempt = time.Now()
	return err
}

func (e *CasbinEnforcer) reloadPolicyLocked() error {
	if e.versionStore == nil {
		return e.Enforcer.LoadPolicy()
	}
	snapshot, version, err := e.versionStore.LoadSnapshot(context.Background(), e.model)
	if err != nil {
		return err
	}
	e.Enforcer = snapshot
	e.policyState.loadedVersion = version
	_ = e.versionStore.RecordLoaded(context.Background(), version)
	return nil
}

// Enforce 串行化策略读取与重载；最新一次重载失败时按 fail-closed 拒绝请求。
func (e *CasbinEnforcer) Enforce(rvals ...interface{}) (bool, error) {
	return e.EnforceContext(context.Background(), rvals...)
}

// EnforceContext 在请求上下文内校验版本并执行权限判断。
func (e *CasbinEnforcer) EnforceContext(ctx context.Context, rvals ...interface{}) (bool, error) {
	if e.policyState == nil {
		return e.Enforcer.Enforce(rvals...)
	}
	if e.versionStore != nil {
		requiredVersion, err := e.versionStore.Current(ctx)
		if err != nil {
			return false, fmt.Errorf("load permission policy version: %w", err)
		}
		if err := e.ensurePolicyVersion(requiredVersion); err != nil {
			return false, err
		}
	} else if err := e.retryFailedReload(); err != nil {
		return false, err
	}
	e.policyState.mu.RLock()
	defer e.policyState.mu.RUnlock()
	if e.policyState.reloadErr != nil {
		return false, fmt.Errorf("casbin policy unavailable after reload failure: %w", e.policyState.reloadErr)
	}
	return e.Enforcer.Enforce(rvals...)
}

func (e *CasbinEnforcer) ensurePolicyVersion(requiredVersion uint64) error {
	e.policyState.mu.Lock()
	defer e.policyState.mu.Unlock()
	if e.policyState.reloadErr == nil && e.policyState.loadedVersion >= requiredVersion {
		return nil
	}
	if e.policyState.reloadErr != nil && time.Since(e.policyState.lastReloadAttempt) < time.Second {
		return fmt.Errorf("casbin policy unavailable after reload failure: %w", e.policyState.reloadErr)
	}
	err := e.reloadPolicyLocked()
	e.policyState.reloadErr = err
	e.policyState.lastReloadAttempt = time.Now()
	if err != nil {
		return fmt.Errorf("reload casbin policy version %d: %w", requiredVersion, err)
	}
	if e.policyState.loadedVersion < requiredVersion {
		err = fmt.Errorf("loaded permission policy version %d is behind required version %d", e.policyState.loadedVersion, requiredVersion)
		e.policyState.reloadErr = err
		return err
	}
	return nil
}

func (e *CasbinEnforcer) retryFailedReload() error {
	e.policyState.mu.RLock()
	retry := e.policyState.reloadErr != nil && time.Since(e.policyState.lastReloadAttempt) >= time.Second
	e.policyState.mu.RUnlock()
	if !retry {
		return nil
	}
	e.policyState.mu.Lock()
	defer e.policyState.mu.Unlock()
	if e.policyState.reloadErr != nil && time.Since(e.policyState.lastReloadAttempt) >= time.Second {
		e.policyState.reloadErr = e.Enforcer.LoadPolicy()
		e.policyState.lastReloadAttempt = time.Now()
	}
	return nil
}

// SetDB 返回一个绑定到指定事务的新 CasbinEnforcer。
func (e *CasbinEnforcer) SetDB(tx *gorm.DB) *CasbinEnforcer {
	return &CasbinEnforcer{
		Enforcer:     e.Enforcer,
		errInit:      e.errInit,
		model:        e.model,
		tx:           tx,
		policyState:  e.policyState,
		versionStore: e.versionStore,
	}
}
