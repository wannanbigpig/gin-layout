package admin_test

import "testing"

const (
	routeScenarioBusiness   = "business"
	routeScenarioContract   = "contract"
	routeScenarioDependency = "dependency"
)

// businessScenarioRoutes 已有成功响应、状态流或真实 MySQL 回读断言。
var businessScenarioRoutes = []string{
	"GET /admin/v1/admin-user/profile-security/options",
	"GET /admin/v1/todo/groups",
	"POST /admin/v1/todo/group/create",
	"POST /admin/v1/todo/group/delete",
	"POST /admin/v1/todo/group/move",
	"POST /admin/v1/todo/group/rename",
	"GET /admin/v1/system/file/upload/options",
	"GET /admin/v1/admin-user/detail",
	"GET /admin/v1/admin-user/get",
	"GET /admin/v1/admin-user/get-full-email",
	"GET /admin/v1/admin-user/get-full-phone",
	"GET /admin/v1/admin-user/list",
	"GET /admin/v1/admin-user/options",
	"GET /admin/v1/admin-user/user-menu-info",
	"POST /admin/v1/admin-user/bind-role",
	"POST /admin/v1/admin-user/create",
	"POST /admin/v1/admin-user/delete",
	"POST /admin/v1/admin-user/update",
	"GET /admin/v1/auth/check-token",
	"POST /admin/v1/auth/logout",
	"POST /admin/v1/auth/refresh",
	"GET /admin/v1/auth/session/list",
	"POST /admin/v1/auth/session/revoke",
	"GET /admin/v1/dashboard/monitor",
	"GET /admin/v1/dashboard/overview",
	"GET /admin/v1/dashboard/personal",
	"GET /admin/v1/dashboard/statistics",
	"POST /admin/v1/department/bind-role",
	"POST /admin/v1/department/create",
	"POST /admin/v1/department/delete",
	"GET /admin/v1/department/detail",
	"GET /admin/v1/department/list",
	"POST /admin/v1/department/update",
	"GET /admin/v1/demo",
	"GET /admin/v1/log/login/list",
	"GET /admin/v1/log/request/list",
	"GET /admin/v1/log/request/self-list",
	"GET /admin/v1/log/request/mask-config",
	"POST /admin/v1/log/request/mask-config",
	"POST /admin/v1/menu/create",
	"POST /admin/v1/menu/delete",
	"GET /admin/v1/menu/detail",
	"GET /admin/v1/menu/list",
	"POST /admin/v1/menu/update",
	"POST /admin/v1/menu/update-all-menu-permissions",
	"GET /admin/v1/permission/list",
	"GET /admin/v1/permission/options",
	"POST /admin/v1/product/create",
	"POST /admin/v1/product/delete",
	"GET /admin/v1/product/detail",
	"GET /admin/v1/product/list",
	"POST /admin/v1/product/update",
	"POST /admin/v1/role/create",
	"POST /admin/v1/role/delete",
	"GET /admin/v1/role/detail",
	"GET /admin/v1/role/list",
	"GET /admin/v1/role/options",
	"POST /admin/v1/role/update",
	"POST /admin/v1/system/config/create",
	"POST /admin/v1/system/config/delete",
	"GET /admin/v1/system/config/detail",
	"GET /admin/v1/system/config/list",
	"POST /admin/v1/system/config/refresh",
	"POST /admin/v1/system/config/update",
	"GET /admin/v1/system/config/value",
	"POST /admin/v1/system/dict/item/create",
	"POST /admin/v1/system/dict/item/delete",
	"GET /admin/v1/system/dict/item/list",
	"POST /admin/v1/system/dict/item/update",
	"GET /admin/v1/system/dict/options",
	"POST /admin/v1/system/dict/type/create",
	"POST /admin/v1/system/dict/type/delete",
	"GET /admin/v1/system/dict/type/detail",
	"GET /admin/v1/system/dict/type/list",
	"POST /admin/v1/system/dict/type/update",
	"GET /admin/v1/system/export/list",
	"GET /admin/v1/system/export/summary",
	"POST /admin/v1/system/file/folder/create",
	"POST /admin/v1/system/file/folder/delete",
	"POST /admin/v1/system/file/folder/move",
	"GET /admin/v1/system/file/folder/stats",
	"GET /admin/v1/system/file/folder/tree",
	"POST /admin/v1/system/file/folder/update",
	"GET /admin/v1/system/file/list",
	"GET /admin/v1/system/file/references",
	"GET /admin/v1/system/file/trash/list",
	"GET /admin/v1/system/notification/list",
	"POST /admin/v1/system/notification/delete",
	"POST /admin/v1/system/notification/read",
	"POST /admin/v1/system/notification/read-all",
	"POST /admin/v1/system/notification/send",
	"GET /admin/v1/system/notification/unread-count",
	"GET /admin/v1/system/storage/config",
	"POST /admin/v1/system/storage/config",
	"POST /admin/v1/system/storage/test",
	"GET /admin/v1/task/cron/state",
	"GET /admin/v1/task/list",
	"GET /admin/v1/task/run/list",
	"GET /admin/v1/task/run/stats",
	"GET /admin/v1/task/run/stats/trend",
	"GET /admin/v1/todo/list",
	"GET /admin/v1/todo/reminder-preference",
	"GET /admin/v1/todo/summary",
	"POST /admin/v1/todo/create",
	"POST /admin/v1/todo/update",
	"POST /admin/v1/todo/status",
	"POST /admin/v1/todo/delete",
	"POST /admin/v1/todo/reminder-preference",
	"GET /admin/v1/login-captcha",
	"GET /admin/v1/login/options",
}

// contractScenarioRoutes 已有明确的参数、NotFound、权限或幂等边界断言，但尚未闭环成功状态流。
var contractScenarioRoutes = []string{
	"POST /admin/v1/admin-user/change-phone",
	"POST /admin/v1/admin-user/profile-security/send-code",
	"POST /admin/v1/admin-user/update-profile",
	"GET /admin/v1/file/:uuid",
	"GET /admin/v1/log/login/detail",
	"GET /admin/v1/log/request/detail",
	"GET /admin/v1/log/request/self-detail",
	"POST /admin/v1/login",
	"GET /admin/v1/system/sms/config",
	"POST /admin/v1/system/sms/config",
	"POST /admin/v1/permission/update",
	"GET /admin/v1/system/export/detail",
	"POST /admin/v1/system/file/batch-delete",
	"POST /admin/v1/system/file/delete",
	"GET /admin/v1/system/file/detail",
	"POST /admin/v1/system/file/move",
	"POST /admin/v1/system/file/trash/batch-destroy",
	"POST /admin/v1/system/file/trash/batch-restore",
	"POST /admin/v1/system/file/trash/destroy",
	"POST /admin/v1/system/file/trash/restore",
	"POST /admin/v1/system/file/update",
	"GET /admin/v1/system/storage/secret",
	"POST /admin/v1/task/operation-config",
	"POST /admin/v1/task/record-policy",
	"POST /admin/v1/task/run/cancel",
	"GET /admin/v1/task/run/detail",
	"GET /admin/v1/task/run/events",
	"POST /admin/v1/task/run/retry",
	"POST /admin/v1/task/trigger",
}

// dependencyScenarioRoutes 需要 Redis、queue、multipart 文件系统或隔离对象存储，当前只验证安全边界。
var dependencyScenarioRoutes = []string{
	"POST /admin/v1/admin-user/avatar/upload",
	"POST /admin/v1/common/upload",
	"GET /admin/v1/common/ws",
	"POST /admin/v1/common/ws-ticket",
	"POST /admin/v1/login/sms",
	"POST /admin/v1/login/sms/send-code",
	"POST /admin/v1/log/request/export",
	"POST /admin/v1/system/file/export",
	"POST /admin/v1/system/file/upload/complete",
	"POST /admin/v1/system/file/upload/complete/batch",
	"POST /admin/v1/system/file/upload/credential",
	"POST /admin/v1/system/file/upload/credential/batch",
	"POST /admin/v1/system/file/upload/local",
	"POST /admin/v1/system/file/upload/multipart/abort",
	"POST /admin/v1/system/file/upload/multipart/complete",
	"POST /admin/v1/system/file/upload/multipart/init",
	"POST /admin/v1/system/sms/test",
}

// TestAllAdminRoutesHaveExplicitScenarioClassification 防止新增路由只有自动 smoke 请求，
// 却没有被明确纳入业务成功、契约边界或外部依赖场景。
func TestAllAdminRoutesHaveExplicitScenarioClassification(t *testing.T) {
	classified := make(map[string]string)
	add := func(kind string, keys []string) {
		t.Helper()
		for _, key := range keys {
			if previous, exists := classified[key]; exists {
				t.Fatalf("接口场景重复分类: %s 同时属于 %s 和 %s", key, previous, kind)
			}
			classified[key] = kind
		}
	}
	add(routeScenarioBusiness, businessScenarioRoutes)
	add(routeScenarioContract, contractScenarioRoutes)
	add(routeScenarioDependency, dependencyScenarioRoutes)

	active := make(map[string]struct{})
	for _, route := range sortedAdminRoutes() {
		key := route.Method + " " + route.Path
		active[key] = struct{}{}
		if _, exists := classified[key]; !exists {
			t.Errorf("声明接口缺少显式业务场景分类: %s", key)
		}
	}

	for key, kind := range classified {
		if _, exists := active[key]; exists {
			continue
		}
		if key == "GET /admin/v1/demo" {
			continue
		}
		t.Errorf("业务场景分类包含已删除或拼写错误的接口: %s (%s)", key, kind)
	}
}
