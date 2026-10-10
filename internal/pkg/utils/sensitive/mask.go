package sensitive

import "strings"

// maskMap 对 map 进行递归脱敏（使用指定的敏感字段列表）
func maskMap(m map[string]interface{}, sensitiveFields map[string]bool) map[string]interface{} {
	if len(m) == 0 {
		return m
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		keyLower := strings.ToLower(k)
		if isSensitiveField(keyLower, sensitiveFields) {
			result[k] = maskValue(v)
		} else {
			result[k] = maskSensitiveDataWithFields(v, sensitiveFields)
		}
	}
	return result
}

// maskSensitiveDataWithFields 对敏感数据进行脱敏处理（使用指定的敏感字段列表）
func maskSensitiveDataWithFields(data interface{}, sensitiveFields map[string]bool) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		return maskMap(v, sensitiveFields)
	case []interface{}:
		return maskArrayWithFields(v, sensitiveFields)
	case string:
		return maskString(v)
	default:
		return data
	}
}

// maskArrayWithFields 对数组进行递归脱敏（使用指定的敏感字段列表）
func maskArrayWithFields(arr []interface{}, sensitiveFields map[string]bool) []interface{} {
	if len(arr) == 0 {
		return arr
	}
	result := make([]interface{}, len(arr))
	for i, v := range arr {
		result[i] = maskSensitiveDataWithFields(v, sensitiveFields)
	}
	return result
}

// IsCommonSensitiveField 判定字段名是否匹配通用敏感字段（包括用户配置与常见隐私凭据）。
func IsCommonSensitiveField(fieldName string) bool {
	fieldName = strings.ToLower(strings.TrimSpace(fieldName))
	if fieldName == "" {
		return false
	}
	return isSensitiveField(fieldName, getRequestBodyFields())
}

// MaskSensitiveValue 对数据执行敏感信息掩码转换。
func MaskSensitiveValue(v any) any {
	return maskValue(v)
}
