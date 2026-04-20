package system

import (
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestResetSystemDataReturnsErrorWhenMysqlUnavailable(t *testing.T) {
	t.Cleanup(func() {
		if err := data.CloseMysql(); err != nil {
			t.Fatalf("close mysql on cleanup: %v", err)
		}
	})
	if err := data.CloseMysql(); err != nil {
		t.Fatalf("close mysql: %v", err)
	}

	err := NewResetService().ResetSystemData()
	if !errors.Is(err, model.ErrDBUninitialized) {
		t.Fatalf("expected %v, got %v", model.ErrDBUninitialized, err)
	}
}

func TestBuildDatabaseURLUsesInjectedConfig(t *testing.T) {
	service := NewResetServiceWithDeps(ResetServiceDeps{
		ConfigProvider: func() *config.Conf {
			return &config.Conf{
				Mysql: autoload.MysqlConfig{
					Host:     "127.0.0.1",
					Port:     3307,
					Database: "demo",
					Username: "tester",
					Password: "secret",
				},
			}
		},
	})

	got := service.buildDatabaseURL()
	want := "mysql://tester:secret@tcp(127.0.0.1:3307)/demo?charset=utf8mb4&parseTime=True&loc=Local"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
