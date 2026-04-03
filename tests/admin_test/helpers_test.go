package admin_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

const testResourcePrefix = "test-auto-"

// requireWritableDB 在需要真实数据库写入时跳过测试。
func requireWritableDB(t *testing.T) {
	t.Helper()
	requireMySQL(t)
	if _, err := model.GetDB(); err != nil {
		t.Skip("数据库连接不可用，跳过真实写入测试")
	}
}

// uniqueTestName 生成用于测试资源的唯一名称。
func uniqueTestName(kind string) string {
	return fmt.Sprintf("%s%s-%d", testResourcePrefix, kind, time.Now().UnixNano())
}

// containsPrefix 判断字符串是否包含测试前缀。
func containsPrefix(s string) bool {
	return strings.HasPrefix(s, testResourcePrefix)
}

// uniqueCompactTestName 生成适合表单校验长度限制的测试名称。
func uniqueCompactTestName(kind string) string {
	return fmt.Sprintf("ta%s%d", strings.ReplaceAll(kind, "-", ""), time.Now().UnixNano()%1e8)
}
