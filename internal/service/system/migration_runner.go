package system

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
)

// ResolveMigrationsPath 解析迁移目录绝对路径。
func ResolveMigrationsPath() (string, error) {
	return getMigrationsPath()
}

// NewMigrator 创建默认迁移执行器（自动解析迁移目录）。
func NewMigrator() (*migrate.Migrate, error) {
	return NewResetService().createMigrateInstance()
}

// NewMigratorWithPath 创建指定迁移目录的迁移执行器。
func NewMigratorWithPath(path string) (*migrate.Migrate, error) {
	trimmedPath := strings.TrimSpace(strings.TrimPrefix(path, "file://"))
	if trimmedPath == "" {
		return nil, fmt.Errorf("迁移目录不能为空")
	}

	absPath, err := filepath.Abs(trimmedPath)
	if err != nil {
		return nil, fmt.Errorf("解析迁移目录失败: %w", err)
	}

	dbURL := NewResetService().buildDatabaseURL()
	m, err := migrate.New(fmt.Sprintf("file://%s", absPath), dbURL)
	if err != nil {
		return nil, fmt.Errorf("创建迁移实例失败: %w", err)
	}
	return m, nil
}
