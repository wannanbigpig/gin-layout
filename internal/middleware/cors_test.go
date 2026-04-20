package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
)

func TestCorsHandlerAllowsWildcardOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restoreConfig := config.ReplaceConfigForTesting(&config.Conf{
		AppConfig: autoload.AppConfig{
			CorsOrigins:     []string{"*"},
			CorsCredentials: false,
		},
	})
	t.Cleanup(restoreConfig)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	engine.Use(CorsHandler())
	engine.GET("/demo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/demo", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	ctx.Request = req

	engine.HandleContext(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard allow origin, got %q", got)
	}
}

func TestCorsHandlerReflectsOriginWhenCredentialsEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restoreConfig := config.ReplaceConfigForTesting(&config.Conf{
		AppConfig: autoload.AppConfig{
			CorsOrigins:     []string{"*"},
			CorsCredentials: true,
		},
	})
	t.Cleanup(restoreConfig)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	engine.Use(CorsHandler())
	engine.GET("/demo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/demo", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	ctx.Request = req

	engine.HandleContext(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected reflected allow origin, got %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header true, got %q", got)
	}
}

func TestCorsHandlerRejectsUnknownOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restoreConfig := config.ReplaceConfigForTesting(&config.Conf{
		AppConfig: autoload.AppConfig{
			CorsOrigins: []string{"https://example.com"},
		},
	})
	t.Cleanup(restoreConfig)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	engine.Use(CorsHandler())
	engine.GET("/demo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/demo", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	ctx.Request = req

	engine.HandleContext(ctx)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}

func TestCorsHandlerAllowsWildcardMethodsAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restoreConfig := config.ReplaceConfigForTesting(&config.Conf{
		AppConfig: autoload.AppConfig{
			CorsOrigins: []string{"*"},
			CorsMethods: []string{"*"},
			CorsHeaders: []string{"*"},
		},
	})
	t.Cleanup(restoreConfig)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	engine.Use(CorsHandler())
	engine.OPTIONS("/demo", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/demo", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	req.Header.Set("Access-Control-Request-Headers", "Authorization,Content-Type")
	ctx.Request = req

	engine.HandleContext(ctx)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS" {
		t.Fatalf("unexpected allow methods: %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "Authorization,Content-Type" {
		t.Fatalf("unexpected allow headers: %q", got)
	}
}

func TestGetExposeHeadersSupportsWildcard(t *testing.T) {
	cfg := &config.Conf{
		AppConfig: autoload.AppConfig{
			CorsExposeHeaders: []string{"*"},
		},
	}

	if got := getExposeHeaders(cfg); got != "*" {
		t.Fatalf("expected wildcard expose headers, got %q", got)
	}
}
