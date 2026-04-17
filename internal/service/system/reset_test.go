package system

import (
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestResetSystemDataReturnsErrorWhenMysqlUnavailable(t *testing.T) {
	originalMysqlEnable := config.Config.Mysql.Enable
	config.Config.Mysql.Enable = false
	defer func() {
		config.Config.Mysql.Enable = originalMysqlEnable
		if err := data.CloseMysql(); err != nil {
			t.Fatalf("close mysql on restore: %v", err)
		}
	}()

	if err := data.CloseMysql(); err != nil {
		t.Fatalf("close mysql: %v", err)
	}

	err := NewResetService().ResetSystemData()
	if !errors.Is(err, model.ErrDBUninitialized) {
		t.Fatalf("expected %v, got %v", model.ErrDBUninitialized, err)
	}
}
