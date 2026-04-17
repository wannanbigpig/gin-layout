package casbinx

import (
	"errors"

	"github.com/casbin/casbin/v3"
	"gorm.io/gorm"
)

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

	txEnforcer, err := newEnforcerFromDB(e.model, tx)
	if err != nil {
		return err
	}
	return fn(txEnforcer)
}

// EditPolicyPermissions 编辑策略权限
func (e *CasbinEnforcer) EditPolicyPermissions(user string, policy [][]string, tx ...*gorm.DB) error {
	return e.execute(firstTx(tx), func(enforcer casbin.IEnforcer) error {
		return replacePermissions(enforcer, user, policy)
	})
}

// EditPolicyPermissionsBatch 批量覆盖多个 subject 的权限策略。
func (e *CasbinEnforcer) EditPolicyPermissionsBatch(subjectPolicies map[string][][]string, tx ...*gorm.DB) error {
	return e.execute(firstTx(tx), func(enforcer casbin.IEnforcer) error {
		for subject, policy := range subjectPolicies {
			if err := replacePermissions(enforcer, subject, policy); err != nil {
				return err
			}
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

func replacePermissions(enforcer casbin.IEnforcer, subject string, policy [][]string) error {
	if _, err := enforcer.DeletePermissionsForUser(subject); err != nil {
		return err
	}

	policies := toSubjectPolicies(subject, policy)
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
}

func toSubjectPolicies(subject string, policy [][]string) [][]string {
	policies := make([][]string, 0, len(policy))
	for _, item := range policy {
		if len(item) > 0 {
			policies = append(policies, append([]string{subject}, item...))
		}
	}
	return policies
}
