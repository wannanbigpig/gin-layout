package resources

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
)

func TestAPITransformerResolvesCatalogMetadataByRequestLocale(t *testing.T) {
	api := &model.Api{
		Name:            "接口列表",
		NameI18n:        i18n.MarshalLocaleMap(map[string]string{i18n.LocaleZhCN: "接口列表", i18n.LocaleEnUS: "API Permission List"}),
		Description:     "接口描述",
		DescriptionI18n: i18n.MarshalLocaleMap(map[string]string{i18n.LocaleZhCN: "接口描述", i18n.LocaleEnUS: "API description"}),
	}

	english := NewApiTransformer().ToLocalizedStruct(api, i18n.LocaleEnUS)
	if english.Name != "API Permission List" || english.Description != "API description" {
		t.Fatalf("unexpected English metadata: %#v", english)
	}

	chinese := NewApiTransformer().ToLocalizedStruct(api, i18n.LocaleZhCN)
	if chinese.Name != api.Name || chinese.Description != api.Description {
		t.Fatalf("Chinese response should preserve compatible fields: %#v", chinese)
	}
}

func TestAPITransformerFallsBackToCompatibleFields(t *testing.T) {
	api := &model.Api{Name: "Custom API", Description: "Custom description"}
	result := NewApiTransformer().ToLocalizedStruct(api, i18n.LocaleEnUS)
	if result.Name != api.Name || result.Description != api.Description {
		t.Fatalf("custom API metadata should fall back to stored fields: %#v", result)
	}
}
