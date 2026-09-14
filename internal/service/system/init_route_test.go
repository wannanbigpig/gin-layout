package system

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/routers"
)

func TestBuildAPIDataFailsWhenRuntimeRouteMissingMetadata(t *testing.T) {
	routes := []gin.RouteInfo{{Method: http.MethodGet, Path: "/missing", Handler: "test.handler"}}
	data, err := buildApiData(routes, routers.RouteMetaMap{})
	if err == nil {
		t.Fatal("expected missing metadata error")
	}
	if data != nil {
		t.Fatalf("expected no partial API data, got %#v", data)
	}
	if !strings.Contains(err.Error(), "GET /missing") {
		t.Fatalf("error should identify missing route, got %v", err)
	}
}
