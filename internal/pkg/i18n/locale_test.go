package i18n

import "testing"

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "empty", header: "", want: LocaleZhCN},
		{name: "english list", header: "en-US,en;q=0.9,zh;q=0.8", want: LocaleEnUS},
		{name: "zh underscore", header: "zh_CN", want: LocaleZhCN},
		{name: "unsupported", header: "fr-FR,fr;q=0.8", want: LocaleZhCN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseAcceptLanguage(tt.header); got != tt.want {
				t.Fatalf("ParseAcceptLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveLocalizedText(t *testing.T) {
	raw := `{"zh-CN":"菜单","en-US":"Menu"}`
	if got := ResolveLocalizedText("默认", raw, LocaleEnUS); got != "Menu" {
		t.Fatalf("expected english text, got %q", got)
	}
	if got := ResolveLocalizedText("默认", raw, "zh_CN"); got != "菜单" {
		t.Fatalf("expected chinese text, got %q", got)
	}
	if got := ResolveLocalizedText("默认", raw, "fr-FR"); got != "菜单" {
		t.Fatalf("expected fallback chinese text, got %q", got)
	}
}

func TestMergeLocaleJSON(t *testing.T) {
	existing := `{"zh-CN":"菜单"}`
	incoming := map[string]string{"en-US": "Menu"}

	raw, title := MergeLocaleJSON(existing, incoming, LocaleEnUS, "Menu")
	if title != "菜单" {
		t.Fatalf("default title should prefer zh-CN, got %q", title)
	}

	localized := ResolveLocalizedText("", raw, LocaleEnUS)
	if localized != "Menu" {
		t.Fatalf("expected merged english text, got %q", localized)
	}
}
