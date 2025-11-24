package sensitive

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

const (
	// 脱敏常量
	maskTokenPrefixLen    = 6 // Token 脱敏保留前缀长度
	maskTokenSuffixLen    = 6 // Token 脱敏保留后缀长度
	maskPhonePrefixLen    = 3 // 手机号脱敏保留前缀长度
	maskPhoneSuffixLen    = 4 // 手机号脱敏保留后缀长度
	maskEmailPrefixLen    = 2 // 邮箱脱敏保留前缀长度
	maskIdCardPrefixLen   = 6 // 身份证脱敏保留前缀长度
	maskIdCardSuffixLen   = 4 // 身份证脱敏保留后缀长度
	maskBankCardPrefixLen = 4 // 银行卡脱敏保留前缀长度
	maskBankCardSuffixLen = 4 // 银行卡脱敏保留后缀长度
	maskDefaultPrefixLen  = 1 // 默认脱敏保留前缀长度
	maskDefaultSuffixLen  = 1 // 默认脱敏保留后缀长度
)

// SensitiveFieldsConfig 敏感字段配置结构（未来可通过配置文件加载）
type SensitiveFieldsConfig struct {
	Common         []string `json:"common"`          // 公共敏感字段（适用于所有场景）
	RequestHeader  []string `json:"request_header"`  // 请求头敏感字段
	RequestBody    []string `json:"request_body"`    // 请求体敏感字段
	ResponseHeader []string `json:"response_header"` // 响应头敏感字段
	ResponseBody   []string `json:"response_body"`   // 响应体敏感字段
}

// sensitiveFieldsManager 敏感字段管理器
type sensitiveFieldsManager struct {
	commonFields         map[string]bool
	requestHeaderFields  map[string]bool
	requestBodyFields    map[string]bool
	responseHeaderFields map[string]bool
	responseBodyFields   map[string]bool
	mu                   sync.RWMutex
}

var (
	// fieldsManager 全局敏感字段管理器
	fieldsManager = &sensitiveFieldsManager{
		commonFields:         make(map[string]bool),
		requestHeaderFields:  make(map[string]bool),
		requestBodyFields:    make(map[string]bool),
		responseHeaderFields: make(map[string]bool),
		responseBodyFields:   make(map[string]bool),
	}

	// 预编译的正则表达式（提升性能）
	phoneRegex    = regexp.MustCompile(`1[3-9]\d{9}`)
	emailRegex    = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	idCardRegex   = regexp.MustCompile(`\d{15}|\d{17}[\dXx]`)
	bankCardRegex = regexp.MustCompile(`\d{16,19}`)
)

// init 初始化默认敏感字段配置
func init() {
	config := SensitiveFieldsConfig{
		Common: []string{
			"password", "pwd", "passwd", "pass", "secret",
			"token", "access_token", "refresh_token",
			"api_key", "apikey", "apiKey",
			"pin", "cvv", "cvc", "cvv2", "security_code",
		},
		RequestHeader: []string{
			"authorization", "auth",
			"cookie",
			"x-api-key", "x-access-token", "x-auth-token", "x-token",
		},
		RequestBody: []string{
			"password", "pwd", "passwd", "pass", "secret",
			"token", "access_token", "refresh_token",
			"api_key", "apikey", "apiKey",
			"phone", "mobile", "tel", "telephone",
			"phone_number", "mobile_number",
			"email", "mail",
			"id_card", "idcard", "identity", "id_number",
			"bank_card", "bankcard", "card_number", "card_no",
			"cvv", "cvc", "cvv2", "security_code",
			"pin", "ssn", "social_security",
			"real_name", "realname", "name",
		},
		ResponseHeader: []string{
			"set-cookie",
			"authorization", "auth",
			"x-api-key", "x-access-token", "x-auth-token", "x-token", "x-refresh-token",
			"refresh-access-token", "refresh-exp",
			"cookie",
		},
		ResponseBody: []string{
			"password", "pwd", "passwd", "pass", "secret",
			"token", "access_token", "refresh_token",
			"api_key", "apikey", "apiKey",
			"phone", "mobile", "tel", "telephone",
			"phone_number", "mobile_number",
			"email", "mail",
			"id_card", "idcard", "identity", "id_number",
			"bank_card", "bankcard", "card_number", "card_no",
			"cvv", "cvc", "cvv2", "security_code",
			"pin", "ssn", "social_security",
		},
	}
	LoadSensitiveFieldsConfig(config)
}

// LoadSensitiveFieldsConfig 加载敏感字段配置（未来可从配置文件调用）
func LoadSensitiveFieldsConfig(config SensitiveFieldsConfig) {
	fieldsManager.mu.Lock()
	defer fieldsManager.mu.Unlock()

	fieldsManager.commonFields = sliceToMap(config.Common)
	fieldsManager.requestHeaderFields = sliceToMap(config.RequestHeader)
	fieldsManager.requestBodyFields = sliceToMap(config.RequestBody)
	fieldsManager.responseHeaderFields = sliceToMap(config.ResponseHeader)
	fieldsManager.responseBodyFields = sliceToMap(config.ResponseBody)
}

// sliceToMap 将字符串切片转换为 map（用于快速查找）
func sliceToMap(slice []string) map[string]bool {
	if len(slice) == 0 {
		return make(map[string]bool)
	}
	result := make(map[string]bool, len(slice))
	for _, s := range slice {
		if s != "" {
			result[strings.ToLower(s)] = true
		}
	}
	return result
}

// getCommonFields 获取公共敏感字段列表（线程安全）
func getCommonFields() map[string]bool {
	fieldsManager.mu.RLock()
	defer fieldsManager.mu.RUnlock()
	// 返回副本，避免外部修改
	result := make(map[string]bool, len(fieldsManager.commonFields))
	for k, v := range fieldsManager.commonFields {
		result[k] = v
	}
	return result
}

// getRequestHeaderFields 获取请求头敏感字段列表（线程安全）
func getRequestHeaderFields() map[string]bool {
	fieldsManager.mu.RLock()
	defer fieldsManager.mu.RUnlock()
	result := make(map[string]bool, len(fieldsManager.requestHeaderFields))
	for k, v := range fieldsManager.requestHeaderFields {
		result[k] = v
	}
	return result
}

// getRequestBodyFields 获取请求体敏感字段列表（线程安全）
func getRequestBodyFields() map[string]bool {
	fieldsManager.mu.RLock()
	defer fieldsManager.mu.RUnlock()
	result := make(map[string]bool, len(fieldsManager.requestBodyFields))
	for k, v := range fieldsManager.requestBodyFields {
		result[k] = v
	}
	return result
}

// getResponseHeaderFields 获取响应头敏感字段列表（线程安全）
func getResponseHeaderFields() map[string]bool {
	fieldsManager.mu.RLock()
	defer fieldsManager.mu.RUnlock()
	result := make(map[string]bool, len(fieldsManager.responseHeaderFields))
	for k, v := range fieldsManager.responseHeaderFields {
		result[k] = v
	}
	return result
}

// getResponseBodyFields 获取响应体敏感字段列表（线程安全）
func getResponseBodyFields() map[string]bool {
	fieldsManager.mu.RLock()
	defer fieldsManager.mu.RUnlock()
	result := make(map[string]bool, len(fieldsManager.responseBodyFields))
	for k, v := range fieldsManager.responseBodyFields {
		result[k] = v
	}
	return result
}

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

// maskString 对字符串进行脱敏（检测并脱敏手机号、邮箱等）
func maskString(s string) string {
	if s == "" {
		return s
	}

	// 按优先级顺序检测和脱敏（从最具体到最通用）
	// 银行卡号（16-19位数字）
	if bankCardRegex.MatchString(s) {
		s = bankCardRegex.ReplaceAllStringFunc(s, maskBankCard)
	}

	// 身份证（15或18位）
	if idCardRegex.MatchString(s) {
		s = idCardRegex.ReplaceAllStringFunc(s, maskIdCard)
	}

	// 手机号（11位）
	if phoneRegex.MatchString(s) {
		s = phoneRegex.ReplaceAllStringFunc(s, maskPhone)
	}

	// 邮箱
	if emailRegex.MatchString(s) {
		s = emailRegex.ReplaceAllStringFunc(s, maskEmail)
	}

	return s
}

// maskValue 对值进行脱敏（根据字段类型）
func maskValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		if val == "" {
			return val
		}
		return maskSensitiveString(val)
	case map[string]interface{}:
		return maskMap(val, getCommonFields())
	case []interface{}:
		return maskArrayWithFields(val, getCommonFields())
	default:
		return v
	}
}

// maskSensitiveString 对敏感字符串进行脱敏
func maskSensitiveString(s string) string {
	if s == "" {
		return s
	}

	// 快速路径：检查常见的前缀模式（避免不必要的字符串操作）
	sLower := strings.ToLower(s)

	// JWT token 格式（以 eyJ 开头）
	if strings.HasPrefix(s, "eyJ") {
		return maskToken(s)
	}

	// Bearer token 格式
	if strings.HasPrefix(sLower, "bearer ") {
		return maskAuthToken(s, "bearer ")
	}

	// Basic auth 格式
	if strings.HasPrefix(sLower, "basic ") {
		return maskAuthToken(s, "basic ")
	}

	// 检测并脱敏各种敏感信息（按优先级）
	if bankCardRegex.MatchString(s) {
		return bankCardRegex.ReplaceAllStringFunc(s, maskBankCard)
	}

	if idCardRegex.MatchString(s) {
		return idCardRegex.ReplaceAllStringFunc(s, maskIdCard)
	}

	if phoneRegex.MatchString(s) {
		return phoneRegex.ReplaceAllStringFunc(s, maskPhone)
	}

	if emailRegex.MatchString(s) {
		return emailRegex.ReplaceAllStringFunc(s, maskEmail)
	}

	// 默认脱敏
	return maskDefault(s)
}

// maskAuthToken 对认证 token 进行脱敏（Bearer/Basic）
func maskAuthToken(s, prefix string) string {
	// 使用传入的 prefix 参数验证并提取 token
	if strings.HasPrefix(strings.ToLower(s), prefix) {
		tokenPart := s[len(prefix):]
		return prefix + maskToken(tokenPart)
	}
	// 如果不匹配，尝试按空格分割（兼容处理）
	parts := strings.SplitN(s, " ", 2)
	if len(parts) == 2 {
		return parts[0] + " " + maskToken(parts[1])
	}
	return maskDefault(s)
}

// isSensitiveField 检查字段名是否包含敏感关键词（优化版本）
func isSensitiveField(fieldName string, sensitiveFields map[string]bool) bool {
	// 快速路径：直接匹配
	if sensitiveFields[fieldName] {
		return true
	}

	// 检查是否包含敏感关键词（优化：只检查长度合理的字段名）
	if len(fieldName) < 3 {
		return false
	}

	// 遍历敏感字段关键词，检查是否包含
	for keyword := range sensitiveFields {
		if len(keyword) <= len(fieldName) && strings.Contains(fieldName, keyword) {
			return true
		}
	}

	return false
}

// maskToken 对 token 进行脱敏（保留前6位和后6位）
func maskToken(token string) string {
	length := len(token)
	if length <= maskTokenPrefixLen+maskTokenSuffixLen {
		return strings.Repeat("*", length)
	}
	return token[:maskTokenPrefixLen] + "***" + token[length-maskTokenSuffixLen:]
}

// maskPhone 对手机号进行脱敏（保留前3位和后4位）
func maskPhone(phone string) string {
	if len(phone) != 11 {
		return maskDefault(phone)
	}
	return phone[:maskPhonePrefixLen] + "****" + phone[11-maskPhoneSuffixLen:]
}

// maskEmail 对邮箱进行脱敏（保留@前2位和@后完整域名）
func maskEmail(email string) string {
	idx := strings.IndexByte(email, '@')
	if idx == -1 || idx == 0 {
		return maskDefault(email)
	}

	localPart := email[:idx]
	domain := email[idx:]

	if len(localPart) <= maskEmailPrefixLen {
		return strings.Repeat("*", len(localPart)) + domain
	}
	return localPart[:maskEmailPrefixLen] + "***" + domain
}

// maskIdCard 对身份证进行脱敏（保留前6位和后4位）
func maskIdCard(idCard string) string {
	length := len(idCard)
	switch length {
	case 15:
		return idCard[:maskIdCardPrefixLen] + "******" + idCard[15-maskIdCardSuffixLen:]
	case 18:
		return idCard[:maskIdCardPrefixLen] + "********" + idCard[18-maskIdCardSuffixLen:]
	default:
		return maskDefault(idCard)
	}
}

// maskBankCard 对银行卡号进行脱敏（保留前4位和后4位）
func maskBankCard(cardNo string) string {
	length := len(cardNo)
	if length < maskBankCardPrefixLen+maskBankCardSuffixLen {
		return maskDefault(cardNo)
	}
	maskLen := length - maskBankCardPrefixLen - maskBankCardSuffixLen
	return cardNo[:maskBankCardPrefixLen] + strings.Repeat("*", maskLen) + cardNo[length-maskBankCardSuffixLen:]
}

// maskDefault 默认脱敏（保留前3位和后3位）
func maskDefault(s string) string {
	length := len(s)
	if length <= maskDefaultPrefixLen+maskDefaultSuffixLen {
		return strings.Repeat("*", length)
	}
	maskLen := length - maskDefaultPrefixLen - maskDefaultSuffixLen
	return s[:maskDefaultPrefixLen] + strings.Repeat("*", maskLen) + s[length-maskDefaultSuffixLen:]
}

// maskHeaders 通用的 header 脱敏处理函数（提取公共逻辑）
func maskHeaders(headers http.Header, sensitiveFields map[string]bool) string {
	if len(headers) == 0 {
		return "{}"
	}

	maskedHeaders := make(http.Header, len(headers))
	for k, v := range headers {
		keyLower := strings.ToLower(k)
		if isSensitiveField(keyLower, sensitiveFields) {
			maskedValues := make([]string, len(v))
			for i, val := range v {
				maskedValues[i] = maskSensitiveString(val)
			}
			maskedHeaders[k] = maskedValues
		} else {
			maskedHeaders[k] = v
		}
	}

	// 序列化为 JSON
	bytes, err := json.Marshal(maskedHeaders)
	if err != nil {
		// 序列化失败，返回空对象
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

	// 检查是否为 JSON 格式
	isJSON := strings.Contains(strings.ToLower(contentType), "application/json")

	// 尝试解析 JSON
	var data interface{}
	if isJSON {
		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			// JSON 解析失败，按字符串处理
			return maskString(string(bodyBytes))
		}
	} else {
		// 非 JSON 格式，直接按字符串处理
		return maskString(string(bodyBytes))
	}

	// 对 JSON 数据进行递归脱敏
	requestBodyFields := getRequestBodyFields()
	maskedData := maskSensitiveDataWithFields(data, requestBodyFields)

	// 重新序列化为 JSON
	maskedBytes, err := json.Marshal(maskedData)
	if err != nil {
		// 序列化失败，返回脱敏后的字符串
		return maskString(string(bodyBytes))
	}

	return string(maskedBytes)
}

// GetMaskedResponseBody 获取脱敏后的响应体
func GetMaskedResponseBody(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}

	// 尝试解析 JSON
	var data interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		// 解析失败，按字符串处理
		return maskString(string(bodyBytes))
	}

	// 对 JSON 数据进行递归脱敏
	responseBodyFields := getResponseBodyFields()
	maskedData := maskSensitiveDataWithFields(data, responseBodyFields)

	// 重新序列化为 JSON
	maskedBytes, err := json.Marshal(maskedData)
	if err != nil {
		// 序列化失败，返回脱敏后的字符串
		return maskString(string(bodyBytes))
	}

	return string(maskedBytes)
}

// MaskQueryString 对查询字符串进行脱敏
func MaskQueryString(queryString string) string {
	if queryString == "" {
		return queryString
	}

	// 解析查询参数
	values, err := url.ParseQuery(queryString)
	if err != nil {
		// 解析失败，按字符串处理
		return maskString(queryString)
	}

	// 对每个参数值进行脱敏
	requestBodyFields := getRequestBodyFields()
	maskedValues := make(url.Values, len(values))

	for k, v := range values {
		keyLower := strings.ToLower(k)
		maskedVals := make([]string, len(v))

		if isSensitiveField(keyLower, requestBodyFields) {
			// 敏感字段，使用敏感字符串脱敏
			for i, val := range v {
				maskedVals[i] = maskSensitiveString(val)
			}
		} else {
			// 非敏感字段，检查值中是否包含敏感信息
			for i, val := range v {
				maskedVals[i] = maskString(val)
			}
		}
		maskedValues[k] = maskedVals
	}

	return maskedValues.Encode()
}
