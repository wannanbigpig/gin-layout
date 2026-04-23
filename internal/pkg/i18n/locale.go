package i18n

import (
	"encoding/json"
	"sort"
	"strings"
)

const (
	LocaleZhCN    = "zh-CN"
	LocaleEnUS    = "en-US"
	DefaultLocale = LocaleZhCN
)

// NormalizeLocale 归一化语言标签，仅支持项目当前定义的语言集合。
func NormalizeLocale(locale string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	if normalized == "" {
		return DefaultLocale
	}

	switch {
	case strings.HasPrefix(normalized, "zh"):
		return LocaleZhCN
	case strings.HasPrefix(normalized, "en"):
		return LocaleEnUS
	default:
		return DefaultLocale
	}
}

// ParseAcceptLanguage 从 Accept-Language 请求头中解析语言。
func ParseAcceptLanguage(headerValue string) string {
	if strings.TrimSpace(headerValue) == "" {
		return DefaultLocale
	}

	items := strings.Split(headerValue, ",")
	for _, item := range items {
		segment := strings.TrimSpace(item)
		if segment == "" {
			continue
		}

		tag := strings.TrimSpace(strings.Split(segment, ";")[0])
		if tag == "" {
			continue
		}
		return NormalizeLocale(tag)
	}

	return DefaultLocale
}

// ParseLocaleMap 将 title_i18n 的 JSON 字符串解析为归一化 map。
func ParseLocaleMap(raw string) map[string]string {
	result := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return result
	}

	parsed := make(map[string]string)
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return result
	}

	for key, value := range parsed {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		result[NormalizeLocale(key)] = trimmedValue
	}
	return result
}

// MarshalLocaleMap 将多语言 map 序列化为 JSON 字符串。
func MarshalLocaleMap(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}

	normalized := make(map[string]string, len(data))
	for key, value := range data {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		normalized[NormalizeLocale(key)] = trimmedValue
	}
	if len(normalized) == 0 {
		return ""
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// ResolveLocalizedText 根据请求语言从多语言文案中解析最终展示文本。
func ResolveLocalizedText(defaultText string, i18nRaw string, locale string) string {
	translations := ParseLocaleMap(i18nRaw)
	if len(translations) > 0 {
		if text := strings.TrimSpace(translations[NormalizeLocale(locale)]); text != "" {
			return text
		}
		if text := strings.TrimSpace(translations[LocaleZhCN]); text != "" {
			return text
		}
		if text := strings.TrimSpace(translations[LocaleEnUS]); text != "" {
			return text
		}

		keys := make([]string, 0, len(translations))
		for key := range translations {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if text := strings.TrimSpace(translations[key]); text != "" {
				return text
			}
		}
	}

	return strings.TrimSpace(defaultText)
}

// MergeLocaleJSON 合并历史与本次提交的多语言文案，并返回持久化 JSON 及默认标题字段值。
func MergeLocaleJSON(existingRaw string, incoming map[string]string, locale string, fallbackTitle string) (string, string) {
	next := ParseLocaleMap(existingRaw)
	for key, value := range incoming {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		next[NormalizeLocale(key)] = trimmedValue
	}

	normalizedLocale := NormalizeLocale(locale)
	trimmedFallback := strings.TrimSpace(fallbackTitle)
	if trimmedFallback != "" {
		if _, exists := next[normalizedLocale]; !exists {
			next[normalizedLocale] = trimmedFallback
		}
	}

	defaultTitle := strings.TrimSpace(next[LocaleZhCN])
	if defaultTitle == "" {
		defaultTitle = strings.TrimSpace(trimmedFallback)
	}
	if defaultTitle == "" {
		defaultTitle = ResolveLocalizedText("", MarshalLocaleMap(next), normalizedLocale)
	}

	return MarshalLocaleMap(next), defaultTitle
}

// ToErrorLanguage 将请求语言转换为错误文案模块使用的语言代码。
func ToErrorLanguage(locale string) string {
	if NormalizeLocale(locale) == LocaleEnUS {
		return "en"
	}
	return "zh_CN"
}
