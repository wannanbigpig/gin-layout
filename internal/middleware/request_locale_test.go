package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
)

func TestRequestLocaleHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestLocaleHandler())
	engine.GET("/demo", func(c *gin.Context) {
		c.String(http.StatusOK, LocaleFromContext(c))
	})

	request := httptest.NewRequest(http.MethodGet, "/demo", nil)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Body.String() != i18n.LocaleEnUS {
		t.Fatalf("expected locale %q, got %q", i18n.LocaleEnUS, recorder.Body.String())
	}
}

func TestLocaleFromContextFallback(t *testing.T) {
	if got := LocaleFromContext(nil); got != i18n.DefaultLocale {
		t.Fatalf("expected default locale %q, got %q", i18n.DefaultLocale, got)
	}
}
