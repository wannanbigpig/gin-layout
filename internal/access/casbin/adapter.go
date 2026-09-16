package casbinx

import (
	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

func newEnforcerFromDB(m model.Model, db *gorm.DB) (*casbin.Enforcer, error) {
	gormadapter.TurnOffAutoMigrate(db)
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, err
	}
	enforcer.EnableAutoSave(true)
	return enforcer, nil
}
