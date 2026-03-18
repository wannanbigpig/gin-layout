package casbinx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
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

// execute 使用共享 Enforcer 或事务级 Enforcer 执行 Casbin 操作。
func (e *CasbinEnforcer) execute(tx *gorm.DB, fn func(enforcer casbin.IEnforcer) error) error {
	if tx == nil {
		tx = e.tx
	}
	if tx == nil {
		return fn(e.Enforcer)
	}
	if !isInTransaction(tx) {
		return errors.New("请先通过 GORM 开启事务")
	}

	gormadapter.TurnOffAutoMigrate(tx)
	txAdapter, err := gormadapter.NewAdapterByDB(tx)
	if err != nil {
		return err
	}

	txEnforcer, err := casbin.NewEnforcer(e.model, txAdapter)
	if err != nil {
		return err
	}
	txEnforcer.EnableAutoSave(true)
	return fn(txEnforcer)
}

// EditPolicyPermissions 编辑策略权限
func (e *CasbinEnforcer) EditPolicyPermissions(user string, policy [][]string, tx ...*gorm.DB) error {
	return e.execute(firstTx(tx), func(enforcer casbin.IEnforcer) error {
		_, err := enforcer.DeletePermissionsForUser(user)
		if err != nil {
			return err
		}
		if len(policy) == 0 {
			return nil
		}

		policies := make([][]string, 0, len(policy))
		for _, p := range policy {
			if len(p) > 0 {
				policies = append(policies, append([]string{user}, p...))
			}
		}
		if len(policies) == 0 {
			return nil
		}

		ok, err := enforcer.AddPolicies(policies)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("添加权限失败")
		}
		return nil
	})
}

// EditPolicyRoles 编辑策略角色
func (e *CasbinEnforcer) EditPolicyRoles(user string, policy []string, tx ...*gorm.DB) error {
	return e.execute(firstTx(tx), func(enforcer casbin.IEnforcer) error {
		_, err := enforcer.DeleteRolesForUser(user)
		if err != nil {
			return err
		}
		if len(policy) == 0 {
			return nil
		}

		rules := make([][]string, 0, len(policy))
		for _, role := range policy {
			if role != "" {
				rules = append(rules, []string{user, role})
			}
		}
		if len(rules) == 0 {
			return nil
		}

		ok, err := enforcer.AddGroupingPolicies(rules)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("添加权限失败~")
		}
		return nil
	})
}

// WithTransaction 在指定事务下执行 Casbin 操作。
func (e *CasbinEnforcer) WithTransaction(tx *gorm.DB, fn func(enforcer casbin.IEnforcer) error) error {
	return e.execute(tx, fn)
}

// firstTx 返回可选事务切片中的第一个事务。
func firstTx(tx []*gorm.DB) *gorm.DB {
	if len(tx) == 0 {
		return nil
	}
	return tx[0]
}

// RegisterCustomFunctions 注册自定义函数
func (e *CasbinEnforcer) registerCustomFunctions() {
	// 注册自定义函数
}

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
