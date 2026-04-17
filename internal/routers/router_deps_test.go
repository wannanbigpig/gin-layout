package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	admin_v1 "github.com/wannanbigpig/gin-layout/internal/controller/admin_v1"
)

// TestAdminRouteTree_WithCustomDeps 测试使用自定义依赖的路由树
func TestAdminRouteTree_WithCustomDeps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 构建依赖容器（全部使用默认实现）
	deps := &ControllerDeps{
		Demo:       controller.NewDemoController(),
		Login:      admin_v1.NewLoginController(),
		Common:     admin_v1.NewCommonController(),
		AdminUser:  admin_v1.NewAdminUserController(),
		Api:        admin_v1.NewApiController(),
		Menu:       admin_v1.NewMenuController(),
		Role:       admin_v1.NewRoleController(),
		Dept:       admin_v1.NewDeptController(),
		RequestLog: admin_v1.NewRequestLogController(),
		LoginLog:   admin_v1.NewAdminLoginLogController(),
	}

	// 构建路由树
	routeTree := AdminRouteTree(deps)

	// 验证路由树非空
	assert.NotNil(t, routeTree)
	assert.Equal(t, "admin/v1", routeTree.Prefix)
}

// TestValidateRouteTree 测试路由树校验
func TestValidateRouteTree(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试正常路由树
	routeTree := AppRouteTree()
	err := ValidateRouteTree(routeTree)
	assert.NoError(t, err)

	// 测试空 Handler 的路由
	invalidTree := RouteGroupDef{
		Prefix: "test",
		Routes: []RouteDef{
			{
				Method:   http.MethodGet,
				Path:     "invalid",
				Handlers: []gin.HandlerFunc{}, // 空 handler
			},
		},
	}
	err = ValidateRouteTree(invalidTree)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handlers registered")
}

// TestAdminRouteTree_DefaultDeps 测试使用默认依赖
func TestAdminRouteTree_DefaultDeps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 传入 nil 应该使用默认依赖
	routeTree := AdminRouteTree(nil)

	assert.NotNil(t, routeTree)
	assert.Equal(t, "admin/v1", routeTree.Prefix)

	// 验证路由树可以正常遍历
	err := ValidateRouteTree(routeTree)
	assert.NoError(t, err)
}

// TestCollectRouteMeta 测试路由元数据收集
func TestCollectRouteMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metaMap := CollectRouteMeta(AppRouteTree())
	assert.NotEmpty(t, metaMap)
}

// BenchmarkAdminRouteTree 性能测试
func BenchmarkAdminRouteTree(b *testing.B) {
	gin.SetMode(gin.TestMode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		routeTree := AdminRouteTree(nil)
		_ = ValidateRouteTree(routeTree)
	}
}

// TestRouterIntegration 集成测试示例
func TestRouterIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试引擎
	engine := gin.New()

	// 使用简化路由树进行测试
	testTree := RouteGroupDef{
		Prefix: "test",
		Routes: []RouteDef{
			{
				Method: http.MethodGet,
				Path:   "ping",
				Auth:   AuthModeNone,
				Handlers: []gin.HandlerFunc{
					func(c *gin.Context) {
						c.String(http.StatusOK, "pong")
					},
				},
			},
		},
	}

	RegisterRoutes(engine, testTree)

	// 发起测试请求
	req, _ := http.NewRequest(http.MethodGet, "/test/ping", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}
