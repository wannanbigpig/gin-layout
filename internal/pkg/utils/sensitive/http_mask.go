package sensitive

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

func maskHeaders(headers http.Header, sensitiveFields map[string]bool) string {
	if len(headers) == 0 {
		return "{}"
	}

	maskedHeaders := make(http.Header, len(headers))
	for k, v := range headers {
		if isSensitiveField(strings.ToLower(k), sensitiveFields) {
			maskedHeaders[k] = maskStringSlice(v, maskSensitiveString)
			continue
		}
		maskedHeaders[k] = v
	}

	bytes, err := json.Marshal(maskedHeaders)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

// GetMaskedRequestHeaders 获取脱敏后的请求头
func GetMaskedRequestHeaders(headers http.Header) string {
	return maskHeaders(headers, getRequestHeaderFields())
}

// GetMaskedResponseHeaders 获取脱敏后的响应头
func GetMaskedResponseHeaders(headers http.Header) string {
	return maskHeaders(headers, getResponseHeaderFields())
}

// GetMaskedRequestBody 获取脱敏后的请求体
func GetMaskedRequestBody(bodyBytes []byte, contentType string) string {
	if len(bodyBytes) == 0 {
		return ""
	}

	contentTypeLower := strings.ToLower(contentType)
	switch {
	case strings.Contains(contentTypeLower, "multipart/form-data"):
		return "[multipart/form-data: file upload, body not logged]"
	case !isValidUTF8(bodyBytes):
		return "[binary data: non-text content, body not logged]"
	case !strings.Contains(contentTypeLower, "application/json"):
		return maskString(string(bodyBytes))
	default:
		return maskJSONBytes(bodyBytes, getRequestBodyFields())
	}
}

// GetMaskedResponseBody 获取脱敏后的响应体
func GetMaskedResponseBody(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}
	if !isValidUTF8(bodyBytes) {
		return "[binary data: non-text content, body not logged]"
	}
	return maskJSONBytes(bodyBytes, getResponseBodyFields())
}

// MaskQueryString 对查询字符串进行脱敏
func MaskQueryString(queryString string) string {
	if queryString == "" {
		return queryString
	}

	values, err := url.ParseQuery(queryString)
	if err != nil {
		return maskString(queryString)
	}

	requestBodyFields := getRequestBodyFields()
	maskedValues := make(url.Values, len(values))
	for key, values := range values {
		maskValueFn := maskString
		if isSensitiveField(strings.ToLower(key), requestBodyFields) {
			maskValueFn = maskSensitiveString
		}
		maskedValues[key] = maskStringSlice(values, maskValueFn)
	}
	// url.Values.Encode 会将脱敏占位符 `*` 编码为 `%2A`，导致审计日志阅读时误以为没有脱敏。
	// `*` 属于 RFC 3986 sub-delims，在 query 中可保留原文，这里还原以提升可读性。
	return strings.ReplaceAll(maskedValues.Encode(), "%2A", "*")
}

func maskJSONBytes(bodyBytes []byte, sensitiveFields map[string]bool) string {
	var data interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return maskString(string(bodyBytes))
	}

	maskedData := maskSensitiveDataWithFields(data, sensitiveFields)
	maskedBytes, err := json.Marshal(maskedData)
	if err != nil {
		return maskString(string(bodyBytes))
	}
	return string(maskedBytes)
}

func maskStringSlice(values []string, fn func(string) string) []string {
	masked := make([]string, len(values))
	for i, value := range values {
		masked[i] = fn(value)
	}
	return masked
}

func isValidUTF8(data []byte) bool {
	return len(data) == 0 || len(string(data)) == len(data) && utf8.Valid(data)
}
