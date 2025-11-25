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

type CasbinEnforcer struct {
	*casbin.Enforcer
	errInit error
	tx      *gorm.DB
	model   model.Model
}

var once sync.Once

var casbinx = &CasbinEnforcer{}

// InitEnforcer 初始化 Casbin Enforcer（仅执行一次）
func InitEnforcer() error {
	once.Do(func() {
		modelPath, err := getModelPath()
		if err != nil {
			casbinx.errInit = err
			return
		}

		m, err := model.NewModelFromFile(modelPath)
		if err != nil {
			casbinx.errInit = fmt.Errorf("加载模型失败: %w", err)
			return
		}
		casbinx.model = m
		// dsn := data.GenerateDSN()
		// adapter, err := gormadapter.NewAdapter("mysql", dsn, true)
		db := data.MysqlDB()
		gormadapter.TurnOffAutoMigrate(data.MysqlDB())
		adapter, err := gormadapter.NewAdapterByDB(db)
		if err != nil {
			casbinx.errInit = fmt.Errorf("创建适配器失败: %w", err)
			return
		}

		enforcer, err := casbin.NewEnforcer(m, adapter)
		if err != nil {
			casbinx.errInit = fmt.Errorf("创建 Enforcer 失败: %w", err)
			return
		}

		// 启用自动保存：策略变更后自动保存到数据库（但不自动加载到内存）
		// 注意：自动保存只保存到数据库，不会自动加载到内存，仍需要手动 LoadPolicy()
		enforcer.EnableAutoSave(true)

		// 可选：启用日志（生产环境建议关闭或使用自定义日志）
		// enforcer.EnableLog(false)  // 默认已启用，生产环境可关闭

		casbinx.Enforcer = enforcer
		casbinx.registerCustomFunctions()
	})
	return casbinx.errInit
}

// GetEnforcer 返回已初始化的 Enforcer 实例
func GetEnforcer() *CasbinEnforcer {
	if casbinx.Enforcer == nil {
		if err := InitEnforcer(); err != nil {
			return nil
		}
	}
	return casbinx
}

func (e *CasbinEnforcer) Error() error {
	return e.errInit
}

func (e *CasbinEnforcer) SetDB(tx *gorm.DB) *CasbinEnforcer {
	e.tx = tx
	return e
}

// WithTransaction 执行事务中的操作
// 注意：这里的事务是指 GORM 事务，不是 Casbin 事务
// 传入的 fc 函数中可以使用 e.Enforcer 进行操作，操作完成后会由外部传入的事务处理提交或回滚操作
// 该事务由外部传入的事务控制，由外部控制提交或回滚，所以应在外部判断事务回滚后重新加载策略（如未开启自动保存，事务commit后也需要重新加载策略）
func (e *CasbinEnforcer) WithTransaction(fc func(e casbin.IEnforcer) error) (err error) {
	a, ok := e.GetAdapter().(*gormadapter.Adapter)
	if !ok {
		return errors.New("适配器类型错误")
	}
	if e.tx != nil {
		if !isInTransaction(e.tx) {
			return errors.New("请先通过 GORM 开启事务后传入 SetDB")
		}
		defer func() {
			// 操作完成后，要重置适配器，否则会导致下次操作时，还是使用当前传入的事务链接（适配器提示sql：transaction has already been committed or rolled back）
			e.SetAdapter(a.Copy())
			// 清理事务状态，防止后续操作使用已关闭的事务
			e.tx = nil
			// 此方法使用外部传入的事务，由外部事务控制提交或回滚，所以应由外部控制重新加载策略
			// 注意：此处使用 e.LoadPolicy() 无意义，仅在此处使用会导致无论事务commit或rollback，他加载的都是未修改前的策略
			// err = e.LoadPolicy()
		}()
		gormadapter.TurnOffAutoMigrate(e.tx)
		var txAdapter *gormadapter.Adapter
		txAdapter, err = gormadapter.NewAdapterByDB(e.tx)
		if err != nil {
			return err
		}
		e.SetAdapter(txAdapter)
	}
	err = fc(e.Enforcer)
	return
}

// isInTransaction 判断是否在事务中
func isInTransaction(db *gorm.DB) bool {
	return db != nil && db.Statement != nil && db.Statement.ConnPool != db.ConnPool
}

// EditPolicyPermissions 编辑策略权限
// 注意：这里的策略是指权限，角色在本系统中指部门、角色、菜单、用户
// 策略格式：[p, 菜单ID, 接口路径, 接口方法]
// 例如：[p, menu:1, /api/v1/user/list, GET] 表示ID为1的菜单可以访问/api/v1/user/list的GET方法
func (e *CasbinEnforcer) EditPolicyPermissions(user string, policy [][]string) error {
	return e.WithTransaction(func(enforcer casbin.IEnforcer) error {
		_, err := enforcer.DeletePermissionsForUser(user)
		if err != nil {
			return err
		}
		if len(policy) == 0 {
			return nil
		}

		// 构建完整的策略规则，每个策略都包含 user
		var policies [][]string
		for _, p := range policy {
			if len(p) > 0 {
				// 策略格式：[user, route, method]
				policies = append(policies, append([]string{user}, p...))
			}
		}

		if len(policies) == 0 {
			return nil
		}

		// 批量添加所有策略
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
// 注意：这里的策略是指权限，角色在本系统中指部门、角色、菜单、用户
// 策略格式：[角色ID, 菜单ID]
// 例如：[g, user:1, dept:1] 表示ID为1的用户可以访问ID为1的部门的所有权限
// 例如：[g, user:1, role:1] 表示ID为1的用户可以访问ID为1的角色的所有权限
// 例如：[g, dept:1, role:1] 表示ID为1的部门可以访问ID为1的角色的所有权限
// 例如：[g, role:1, role:2] 表示ID为1的角色可以访问ID为2的角色的所有权限
// 例如：[g, role:2, menu:1] 表示ID为2的角色可以访问ID为1的菜单的所有权限
func (e *CasbinEnforcer) EditPolicyRoles(user string, policy []string) error {
	return e.WithTransaction(func(enforcer casbin.IEnforcer) error {
		_, err := enforcer.DeleteRolesForUser(user)
		if err != nil {
			return err
		}
		if len(policy) == 0 {
			return nil
		}

		// 构建完整的策略规则，每个策略都包含 user 和 role
		var rules [][]string
		for _, role := range policy {
			if role != "" {
				// 策略格式：[user, role]
				rules = append(rules, []string{user, role})
			}
		}

		if len(rules) == 0 {
			return nil
		}

		// 批量添加所有策略
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

// RegisterCustomFunctions 注册自定义函数
func (e *CasbinEnforcer) registerCustomFunctions() {
	// 注册自定义函数
}

// getModelPath 获取 rbac_model.conf 路径并校验是否存在
func getModelPath() (string, error) {
	path := filepath.Join(c.Config.BasePath, "rbac_model.conf")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("模型文件不存在: %s", path)
	}
	return path, nil
}
