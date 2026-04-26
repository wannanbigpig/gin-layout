package auditdiff

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ChangeDiffItem 表示单个字段变更。
type ChangeDiffItem struct {
	Field         string `json:"field"`
	Label         string `json:"label,omitempty"`
	Before        any    `json:"before,omitempty"`
	After         any    `json:"after,omitempty"`
	BeforeDisplay string `json:"before_display,omitempty"`
	AfterDisplay  string `json:"after_display,omitempty"`
}

// FieldRule 描述字段 diff 规则。
type FieldRule struct {
	Field       string
	Label       string
	ValueLabels map[string]string
	Formatter   func(value any) string
}

// BuildFieldDiff 按字段规则构建 before/after 差异。
func BuildFieldDiff(before, after map[string]any, rules []FieldRule) []ChangeDiffItem {
	if len(rules) == 0 {
		return nil
	}
	result := make([]ChangeDiffItem, 0, len(rules))
	for _, rule := range rules {
		if strings.TrimSpace(rule.Field) == "" {
			continue
		}
		beforeValue, hasBefore := before[rule.Field]
		afterValue, hasAfter := after[rule.Field]
		if !hasBefore && !hasAfter {
			continue
		}
		if valuesEqual(beforeValue, afterValue) {
			continue
		}

		item := ChangeDiffItem{
			Field: rule.Field,
			Label: strings.TrimSpace(rule.Label),
		}
		if item.Label == "" {
			item.Label = rule.Field
		}
		if hasBefore {
			item.Before = beforeValue
			item.BeforeDisplay = formatDisplayValue(rule, beforeValue)
		}
		if hasAfter {
			item.After = afterValue
			item.AfterDisplay = formatDisplayValue(rule, afterValue)
		}
		result = append(result, item)
	}
	return result
}

// Marshal 将 diff 项编码为 JSON 字符串；空 diff 返回 []。
func Marshal(items []ChangeDiffItem) string {
	if len(items) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func formatDisplayValue(rule FieldRule, value any) string {
	if rule.Formatter != nil {
		return strings.TrimSpace(rule.Formatter(value))
	}
	if len(rule.ValueLabels) == 0 {
		return ""
	}
	key := valueLabelKey(value)
	if key == "" {
		return ""
	}
	return strings.TrimSpace(rule.ValueLabels[key])
}

func valueLabelKey(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func valuesEqual(before, after any) bool {
	if reflect.DeepEqual(before, after) {
		return true
	}
	beforeKey := valueLabelKey(before)
	afterKey := valueLabelKey(after)
	if beforeKey != "" || afterKey != "" {
		return beforeKey == afterKey
	}
	beforeRaw, beforeErr := json.Marshal(before)
	afterRaw, afterErr := json.Marshal(after)
	if beforeErr != nil || afterErr != nil {
		return false
	}
	return string(beforeRaw) == string(afterRaw)
}
