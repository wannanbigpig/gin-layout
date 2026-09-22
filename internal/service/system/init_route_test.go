package system

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
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

func TestBuildAPIDataPersistsCatalogLocalizationSnapshots(t *testing.T) {
	routes := []gin.RouteInfo{{Method: http.MethodGet, Path: "/example", Handler: "test.handler"}}
	code := utils.MD5(http.MethodGet + "_/example")
	meta := routers.RouteMetaMap{
		code: {
			Method:    http.MethodGet,
			Path:      "/example",
			Title:     "示例",
			Desc:      "示例描述",
			TitleI18n: map[string]string{i18n.LocaleZhCN: "示例", i18n.LocaleEnUS: "Example"},
			DescI18n:  map[string]string{i18n.LocaleZhCN: "示例描述", i18n.LocaleEnUS: "Example description"},
		},
	}

	data, err := buildApiData(routes, meta)
	if err != nil {
		t.Fatalf("build API data: %v", err)
	}
	if len(data) != 1 {
		t.Fatalf("unexpected API row count: %d", len(data))
	}
	if got := i18n.ParseLocaleMap(data[0]["name_i18n"].(string))[i18n.LocaleEnUS]; got != "Example" {
		t.Fatalf("unexpected English name snapshot: %q", got)
	}
	if got := i18n.ParseLocaleMap(data[0]["description_i18n"].(string))[i18n.LocaleEnUS]; got != "Example description" {
		t.Fatalf("unexpected English description snapshot: %q", got)
	}
}
