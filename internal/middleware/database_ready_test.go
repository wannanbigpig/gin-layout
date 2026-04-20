package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

type dependencyErrorResponse struct {
	Code int `json:"code"`
}

func TestDatabaseReadyGuardBlocksWhenMysqlUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreMysql := disableMysqlForGuardTest(t)
	defer restoreMysql()

	engine := gin.New()
	nextCalled := false
	engine.Use(DatabaseReadyGuard())
	engine.GET("/guarded", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	engine.ServeHTTP(recorder, request)

	if nextCalled {
		t.Fatal("expected strict guard to block request when mysql is unavailable")
	}
	assertDependencyNotReadyCode(t, recorder)
}

func TestOptionalDatabaseReadyGuardKeepsUnauthenticatedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreMysql := disableMysqlForGuardTest(t)
	defer restoreMysql()

	engine := gin.New()
	nextCalled := false
	engine.Use(OptionalDatabaseReadyGuard())
	engine.GET("/guarded", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	engine.ServeHTTP(recorder, request)

	if !nextCalled {
		t.Fatal("expected optional guard to skip unauthenticated request")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

func TestOptionalDatabaseReadyGuardBlocksAuthenticatedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreMysql := disableMysqlForGuardTest(t)
	defer restoreMysql()

	engine := gin.New()
	nextCalled := false
	engine.Use(func(c *gin.Context) {
		c.Set(global.ContextKeyUID, uint(1))
		c.Next()
	})
	engine.Use(OptionalDatabaseReadyGuard())
	engine.GET("/guarded", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	engine.ServeHTTP(recorder, request)

	if nextCalled {
		t.Fatal("expected optional guard to block authenticated request when mysql is unavailable")
	}
	assertDependencyNotReadyCode(t, recorder)
}

func disableMysqlForGuardTest(t *testing.T) func() {
	t.Helper()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Mysql.Enable = false
	})
	if err := data.CloseMysql(); err != nil {
		t.Fatalf("close mysql: %v", err)
	}

	return func() {
		restoreConfig()
		if err := data.CloseMysql(); err != nil {
			t.Fatalf("close mysql: %v", err)
		}
	}
}

func assertDependencyNotReadyCode(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var result dependencyErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Code != e.ServiceDependencyNotReady {
		t.Fatalf("expected code %d, got %d", e.ServiceDependencyNotReady, result.Code)
	}
}
