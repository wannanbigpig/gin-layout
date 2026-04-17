package casbinx

import (
	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

func newAdapter(db *gorm.DB) (*gormadapter.Adapter, error) {
	gormadapter.TurnOffAutoMigrate(db)
	return gormadapter.NewAdapterByDB(db)
}

func newEnforcerFromDB(m model.Model, db *gorm.DB) (*casbin.Enforcer, error) {
	adapter, err := newAdapter(db)
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
