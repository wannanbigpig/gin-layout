package request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetAccessTokenSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx.Request.Header.Set("Authorization", "Bearer token-value")

	tokenValue, err := GetAccessToken(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tokenValue != "token-value" {
		t.Fatalf("unexpected token value %q", tokenValue)
	}
}

func TestGetAccessTokenNilContext(t *testing.T) {
	_, err := GetAccessToken(nil)
	if err == nil {
		t.Fatal("expected nil context to return error")
	}
}

func TestGetQueryParamsNilContext(t *testing.T) {
	params := GetQueryParams(nil)
	if len(params) != 0 {
		t.Fatalf("expected empty map, got %#v", params)
	}
}
