package access

import (
	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"gorm.io/gorm"
)

func defaultReloadPolicy() error {
	return casbinx.ReloadPolicy()
}

// getPolicyEnforcer 返回已初始化的 Casbin 封装实例。
func getPolicyEnforcer() (*casbinx.CasbinEnforcer, error) {
	enforcer, err := casbinx.GetEnforcer()
	if err != nil {
		return nil, e.NewBusinessError(e.CasbinInitFailed)
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
