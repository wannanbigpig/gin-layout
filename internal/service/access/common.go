package access

import (
	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"gorm.io/gorm"
)

// getPolicyEnforcer 返回已初始化的 Casbin 封装实例。
func getPolicyEnforcer() (*casbinx.CasbinEnforcer, error) {
	enforcer, err := casbinx.GetEnforcer()
	if err != nil {
		return nil, e.NewBusinessError(1, "casbin 初始化失败")
	}
	return enforcer, nil
}

// FirstTx 返回可选事务切片中的第一个事务。
func FirstTx(tx []*gorm.DB) *gorm.DB {
	if len(tx) == 0 {
		return nil
	}
	return tx[0]
}
