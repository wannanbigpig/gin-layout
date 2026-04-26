package service

import (
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
)

// NormalizeLocaleTextMap 规范化多语言文本输入，保留未来可扩展语言。
func NormalizeLocaleTextMap(data map[string]string) map[string]string {
	result := make(map[string]string, len(data))
	for locale, text := range data {
		normalizedLocale := NormalizeLocaleKey(locale)
		trimmedText := strings.TrimSpace(text)
		if normalizedLocale == "" || trimmedText == "" {
			continue
		}
		result[normalizedLocale] = trimmedText
	}
	return result
}

// NormalizeLocaleKey 统一 zh/en 的历史写法，并保留其他语言原值。
func NormalizeLocaleKey(locale string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(locale, "_", "-"))
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lower, "zh"):
		return i18n.LocaleZhCN
	case strings.HasPrefix(lower, "en"):
		return i18n.LocaleEnUS
	default:
		return trimmed
	}
}

// LocalePriority 返回读路径使用的语言优先级。
func LocalePriority(locale string) []string {
	candidates := []string{
		NormalizeLocaleKey(locale),
		i18n.DefaultLocale,
		i18n.LocaleEnUS,
	}
	result := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		result = append(result, candidate)
	}
	return result
}

// ResolveLocaleText 根据优先级解析展示文本。
func ResolveLocaleText(translations map[string]string, locale string) string {
	for _, candidate := range LocalePriority(locale) {
		if text := strings.TrimSpace(translations[candidate]); text != "" {
			return text
		}
	}
	for _, text := range translations {
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
