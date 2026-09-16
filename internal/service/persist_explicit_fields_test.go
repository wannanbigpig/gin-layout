package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreatePreservesExplicitDisabledFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		value  any
		table  string
		fields map[string]any
	}{
		{"dict type", model.NewSysDictType(), "sys_dict_type", map[string]any{"status": uint8(0)}},
		{"dict item", model.NewSysDictItem(), "sys_dict_item", map[string]any{"status": uint8(0)}},
		{"role", model.NewRole(), "role", map[string]any{"status": uint8(0)}},
		{"product", model.NewProduct(), "product", map[string]any{"status": uint8(0)}},
		{"config", model.NewSysConfig(), "sys_config", map[string]any{"status": uint8(0), "is_visible": uint8(0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			defer sqlDB.Close()
			if err := createExplicitFieldsTestTable(db, tc.table); err != nil {
				t.Fatal(err)
			}
			if err := CreateWithExplicitFields(db, tc.value, tc.fields); err != nil {
				t.Fatal(err)
			}
			var got struct {
				Status    uint8
				IsVisible uint8
			}
			if err := db.Table(tc.table).First(&got).Error; err != nil {
				t.Fatal(err)
			}
			if got.Status != 0 {
				t.Fatalf("status=%d", got.Status)
			}
			if tc.table == "sys_config" && got.IsVisible != 0 {
				t.Fatalf("is_visible=%d", got.IsVisible)
			}
			stmt := &gorm.Statement{DB: db}
			if err := stmt.Parse(tc.value); err != nil {
				t.Fatal(err)
			}
			// The saved model must agree with the database after GORM applied defaults.
			field := stmt.Schema.LookUpField("Status")
			value, _ := field.ValueOf(context.Background(), reflect.ValueOf(tc.value))
			if value != uint8(0) {
				t.Fatalf("model status=%v", value)
			}
		})
	}
}

func TestCreateExplicitFieldsFailureRollsBack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := createExplicitFieldsTestTable(db, "sys_dict_type"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER reject_status BEFORE UPDATE ON sys_dict_type BEGIN SELECT RAISE(ABORT, 'rejected'); END;").Error; err != nil {
		t.Fatal(err)
	}
	if err := CreateWithExplicitFields(db, model.NewSysDictType(), map[string]any{"status": 0}); err == nil {
		t.Fatal("expected failed update")
	}
	var count int64
	if err := db.Model(&model.SysDictType{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("insert was not rolled back: %d", count)
	}
}

func createExplicitFieldsTestTable(db *gorm.DB, table string) error {
	columns := map[string]string{
		"sys_dict_type": "type_code text, is_system integer, status integer default 1, sort integer, remark text",
		"sys_dict_item": "type_code text, value text, color text, tag_type text, is_default integer, is_system integer, status integer default 1, sort integer, remark text",
		"role":          "code text, is_system integer, name text, description text, sort integer, status integer default 1, data_scope integer",
		"product":       "name text, description text, price integer, status integer default 1, dept_id integer, created_by integer",
		"sys_config":    "config_key text, config_value text, value_type text, group_code text, is_system integer, is_sensitive integer, is_visible integer default 1, manage_tab text, status integer default 1, sort integer, remark text",
	}
	return db.Exec("CREATE TABLE " + table + " (id integer primary key autoincrement, created_at datetime, updated_at datetime, deleted_at integer default 0, " + columns[table] + ")").Error
}
