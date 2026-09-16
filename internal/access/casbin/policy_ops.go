package casbinx

import (
	"errors"
	"fmt"

	"github.com/casbin/casbin/v3"
	"gorm.io/gorm"
)

// newEnforcerFromDBFunc 可替换的 Enforcer 构建函数，测试时注入 mock。
var newEnforcerFromDBFunc = newEnforcerFromDB

// execute 使用共享 Enforcer 或事务级 Enforcer 执行 Casbin 操作。
func (e *CasbinEnforcer) execute(tx *gorm.DB, fn func(enforcer casbin.IEnforcer) error) error {
	if tx == nil {
		tx = e.tx
	}
	if tx == nil {
		if e.policyState != nil {
			e.policyState.mu.Lock()
			defer e.policyState.mu.Unlock()
			if e.policyState.reloadErr != nil {
				return fmt.Errorf("casbin policy unavailable after reload failure: %w", e.policyState.reloadErr)
			}
		}
		return fn(e.Enforcer)
	}
	if !isInTransaction(tx) {
		return errors.New("请先通过 GORM 开启事务")
	}

	if e.txEnforcerCache != nil {
		if cached, ok := e.txEnforcerCache[tx]; ok {
			return fn(cached)
		}
	}

	txEnforcer, err := newEnforcerFromDBFunc(e.model, tx)
	if err != nil {
		return err
	}

	// 该缓存的生命周期绑定到当前 enforcer 实例：事务级调用应经 SetDB(tx) 取一次性 wrapper，
	// 用完即随 wrapper 回收，故不会随事务无限增长。切勿在长生命周期的全局单例上直接传入裸 tx，
	// 否则条目无人清理会泄漏（参见 user_permission_sync.go 统一走 SetDB(tx)）。
	if e.txEnforcerCache == nil {
		e.txEnforcerCache = make(map[*gorm.DB]*casbin.Enforcer)
	}
	e.txEnforcerCache[tx] = txEnforcer

	return fn(txEnforcer)
}

// EditPolicyPermissions 编辑策略权限
func (e *CasbinEnforcer) EditPolicyPermissions(user string, policy [][]string, tx ...*gorm.DB) error {
	if execTx := firstTxOrBound(tx, e.tx); execTx != nil {
		if !isInTransaction(execTx) {
			return errors.New("请先通过 GORM 开启事务")
		}
		return replacePermissionsInDB(execTx, map[string][][]string{user: policy})
	}
	return e.execute(firstTx(tx), func(enforcer casbin.IEnforcer) error {
		return replacePermissions(enforcer, user, policy)
	})
}

// EditPolicyPermissionsBatch 批量覆盖多个 subject 的权限策略。
func (e *CasbinEnforcer) EditPolicyPermissionsBatch(subjectPolicies map[string][][]string, tx ...*gorm.DB) error {
	if execTx := firstTxOrBound(tx, e.tx); execTx != nil {
		if !isInTransaction(execTx) {
			return errors.New("请先通过 GORM 开启事务")
		}
		return replacePermissionsInDB(execTx, subjectPolicies)
	}
	return e.execute(firstTx(tx), func(enforcer casbin.IEnforcer) error {
		for subject, policy := range subjectPolicies {
			if err := replacePermissions(enforcer, subject, policy); err != nil {
				return err
			}
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

func firstTxOrBound(tx []*gorm.DB, bound *gorm.DB) *gorm.DB {
	if execTx := firstTx(tx); execTx != nil {
		return execTx
	}
	return bound
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

type casbinPolicyRule struct {
	Ptype string `gorm:"column:ptype"`
	V0    string `gorm:"column:v0"`
	V1    string `gorm:"column:v1"`
	V2    string `gorm:"column:v2"`
	V3    string `gorm:"column:v3"`
	V4    string `gorm:"column:v4"`
	V5    string `gorm:"column:v5"`
}

func (casbinPolicyRule) TableName() string {
	return "casbin_rule"
}

func replacePermissionsInDB(tx *gorm.DB, subjectPolicies map[string][][]string) error {
	subjects := make([]string, 0, len(subjectPolicies))
	for subject := range subjectPolicies {
		if subject != "" {
			subjects = append(subjects, subject)
		}
	}
	if len(subjects) == 0 {
		return nil
	}
	if err := tx.Where("ptype = ? AND v0 IN ?", "p", subjects).Delete(&casbinPolicyRule{}).Error; err != nil {
		return err
	}

	rows := make([]casbinPolicyRule, 0)
	seen := make(map[[7]string]struct{})
	for subject, policy := range subjectPolicies {
		for _, row := range buildCasbinPolicyRows(subject, policy) {
			key := [7]string{row.Ptype, row.V0, row.V1, row.V2, row.V3, row.V4, row.V5}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func buildCasbinPolicyRows(subject string, policy [][]string) []casbinPolicyRule {
	if subject == "" {
		return nil
	}
	rows := make([]casbinPolicyRule, 0, len(policy))
	for _, item := range policy {
		if len(item) == 0 {
			continue
		}
		row := casbinPolicyRule{Ptype: "p", V0: subject}
		if len(item) > 0 {
			row.V1 = item[0]
		}
		if len(item) > 1 {
			row.V2 = item[1]
		}
		if len(item) > 2 {
			row.V3 = item[2]
		}
		if len(item) > 3 {
			row.V4 = item[3]
		}
		if len(item) > 4 {
			row.V5 = item[4]
		}
		rows = append(rows, row)
	}
	return rows
}
