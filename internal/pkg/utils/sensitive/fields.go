package sensitive

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	maskTokenPrefixLen    = 6
	maskTokenSuffixLen    = 6
	maskPhonePrefixLen    = 3
	maskPhoneSuffixLen    = 4
	maskEmailPrefixLen    = 2
	maskIdCardPrefixLen   = 6
	maskIdCardSuffixLen   = 4
	maskBankCardPrefixLen = 4
	maskBankCardSuffixLen = 4
	maskDefaultPrefixLen  = 1
	maskDefaultSuffixLen  = 1
	// 超过该长度的 maskDefault 输出将折叠中间的 `*` 为固定 `***`，避免一长串星号污染审计日志可读性。
	maskDefaultFoldThreshold = 32
)

// SensitiveFieldsConfig 敏感字段配置结构（未来可通过配置文件加载）
type SensitiveFieldsConfig struct {
	Common         []string `json:"common"`
	RequestHeader  []string `json:"request_header"`
	RequestBody    []string `json:"request_body"`
	ResponseHeader []string `json:"response_header"`
	ResponseBody   []string `json:"response_body"`
}

type sensitiveFieldsManager struct {
	commonFields         map[string]bool
	requestHeaderFields  map[string]bool
	requestBodyFields    map[string]bool
	responseHeaderFields map[string]bool
	responseBodyFields   map[string]bool
	mu                   sync.RWMutex
}

var (
	defaultFieldsManagerOnce sync.Once
	defaultFieldsManagerVal  atomic.Pointer[sensitiveFieldsManager]

	phoneRegex    = regexp.MustCompile(`1[3-9]\d{9}`)
	emailRegex    = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	idCardRegex   = regexp.MustCompile(`\d{15}|\d{17}[\dXx]`)
	bankCardRegex = regexp.MustCompile(`\d{16,19}`)
)

// immutableCredentialFields applies to every audit scope, independently of
// editable business-field configuration. Provider registrations only add keys.
var immutableCredentialFields = map[string]bool{
	"password": true, "pwd": true, "passwd": true, "pass": true,
	"secret": true, "token": true, "access_token": true, "refresh_token": true,
	"api_key": true, "apikey": true,
	"authorization": true, "auth": true, "cookie": true, "set-cookie": true,
	"currentpassword": true, "newpassword": true, "accesstoken": true, "refreshtoken": true,
	"clientsecret": true, "secretkey": true, "x-api-key": true,
	"pin": true, "cvv": true, "cvc": true, "cvv2": true, "security_code": true,
	"sms_code": true, "otp": true, "captcha": true, "code_hash": true,
	"access_key_secret": true, "secret_access_key": true,
}

// immutableRequestBodyFields 是认证凭据和付费渠道密钥的安全基线。
// audit.sensitive_fields 可以追加业务字段，但不能让这些字段恢复为明文日志。
var immutableRequestBodyFields = map[string]bool{
	"access_key_secret": true,
	"secret_access_key": true,
	"sms_code":          true,
	"otp":               true,
	"captcha":           true,
	"code_hash":         true,
	"phone":             true,
	"mobile":            true,
	"phone_number":      true,
	"mobile_number":     true,
}

var immutableRequestBodyFieldsMu sync.RWMutex

// RegisterImmutableRequestBodyFields lets compile-time provider registries add
// credential field names that audit logging must never expose in any scope. Registration
// is additive so runtime audit configuration cannot remove the safety baseline.
func RegisterImmutableRequestBodyFields(fields ...string) {
	immutableRequestBodyFieldsMu.Lock()
	defer immutableRequestBodyFieldsMu.Unlock()
	for _, field := range fields {
		field = strings.ToLower(strings.TrimSpace(field))
		if field != "" {
			immutableRequestBodyFields[field] = true
			immutableCredentialFields[field] = true
		}
	}
}

func currentFieldsManager() *sensitiveFieldsManager {
	defaultFieldsManagerOnce.Do(func() {
		defaultFieldsManagerVal.Store(newSensitiveFieldsManager(defaultSensitiveFieldsConfig()))
	})

	manager := defaultFieldsManagerVal.Load()
	if manager != nil {
		return manager
	}

	// 防御性兜底，确保任何情况下都返回可用 manager。
	manager = newSensitiveFieldsManager(defaultSensitiveFieldsConfig())
	defaultFieldsManagerVal.Store(manager)
	return manager
}

func newSensitiveFieldsManager(config SensitiveFieldsConfig) *sensitiveFieldsManager {
	manager := &sensitiveFieldsManager{
		commonFields:         make(map[string]bool),
		requestHeaderFields:  make(map[string]bool),
		requestBodyFields:    make(map[string]bool),
		responseHeaderFields: make(map[string]bool),
		responseBodyFields:   make(map[string]bool),
	}
	manager.applyConfig(config)
	return manager
}

func (m *sensitiveFieldsManager) applyConfig(config SensitiveFieldsConfig) {
	normalized := NormalizeSensitiveFieldsConfig(config)
	m.commonFields = sliceToMap(normalized.Common)
	m.requestHeaderFields = mergeFieldMaps(m.commonFields, normalized.RequestHeader)
	m.requestBodyFields = mergeFieldMaps(m.commonFields, normalized.RequestBody)
	m.responseHeaderFields = mergeFieldMaps(m.commonFields, normalized.ResponseHeader)
	m.responseBodyFields = mergeFieldMaps(m.commonFields, normalized.ResponseBody)
}

func defaultSensitiveFieldsConfig() SensitiveFieldsConfig {
	return NormalizeSensitiveFieldsConfig(SensitiveFieldsConfig{
		Common: []string{
			"password", "pwd", "passwd", "pass", "secret",
			"token", "access_token", "refresh_token",
			"api_key", "apikey", "apiKey",
			"pin", "cvv", "cvc", "cvv2", "security_code",
			"sms_code", "otp", "captcha", "code_hash",
		},
		RequestHeader: []string{
			"authorization", "auth",
			"cookie",
			"x-api-key", "x-access-token", "x-auth-token", "x-token",
		},
		RequestBody: []string{
			"phone", "mobile", "tel", "telephone",
			"phone_number", "mobile_number",
			"email", "mail",
			"id_card", "idcard", "identity", "id_number",
			"bank_card", "bankcard", "card_number", "card_no",
			"ssn", "social_security",
			// 仅保留显式表达“真实姓名”的字段；通用 name 字段误报率高，已移出默认列表，
			// 业务确有脱敏需要时可通过 LoadSensitiveFieldsConfig 补充。
			"real_name", "realname",
		},
		ResponseHeader: []string{
			"set-cookie",
			"authorization", "auth",
			"x-api-key", "refresh-exp",
			"cookie",
		},
		ResponseBody: []string{
			"phone", "mobile", "tel", "telephone",
			"phone_number", "mobile_number",
			"email", "mail",
			"id_card", "idcard", "identity", "id_number",
			"bank_card", "bankcard", "card_number", "card_no",
			"ssn", "social_security",
		},
	})
}

// DefaultSensitiveFieldsConfig 返回默认敏感字段配置副本。
func DefaultSensitiveFieldsConfig() SensitiveFieldsConfig {
	return defaultSensitiveFieldsConfig()
}

// LoadSensitiveFieldsConfig 加载敏感字段配置（未来可从配置文件调用）
func LoadSensitiveFieldsConfig(config SensitiveFieldsConfig) {
	// Initialize once before publishing, then replace the entire immutable snapshot.
	// Concurrent readers must never observe map fields being replaced in place.
	_ = currentFieldsManager()
	defaultFieldsManagerVal.Store(newSensitiveFieldsManager(config))
}

// GetSensitiveFieldsConfig 返回当前生效的敏感字段配置快照。
func GetSensitiveFieldsConfig() SensitiveFieldsConfig {
	manager := currentFieldsManager()
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	return SensitiveFieldsConfig{
		Common:         mapKeys(manager.commonFields),
		RequestHeader:  subtractCommonFieldKeys(manager.requestHeaderFields, manager.commonFields),
		RequestBody:    subtractCommonFieldKeys(manager.requestBodyFields, manager.commonFields),
		ResponseHeader: subtractCommonFieldKeys(manager.responseHeaderFields, manager.commonFields),
		ResponseBody:   subtractCommonFieldKeys(manager.responseBodyFields, manager.commonFields),
	}
}

// NormalizeSensitiveFieldsConfig 统一清洗敏感字段配置，并移除已在 common 中声明的分组重复字段。
func NormalizeSensitiveFieldsConfig(config SensitiveFieldsConfig) SensitiveFieldsConfig {
	common := normalizeFieldList(config.Common, nil)
	commonSet := sliceToMap(common)
	return SensitiveFieldsConfig{
		Common:         common,
		RequestHeader:  normalizeFieldList(config.RequestHeader, commonSet),
		RequestBody:    normalizeFieldList(config.RequestBody, commonSet),
		ResponseHeader: normalizeFieldList(config.ResponseHeader, commonSet),
		ResponseBody:   normalizeFieldList(config.ResponseBody, commonSet),
	}
}

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

func normalizeFieldList(input []string, exclude map[string]bool) []string {
	if len(input) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		trimmed := strings.ToLower(strings.TrimSpace(item))
		if trimmed == "" {
			continue
		}
		if exclude != nil && exclude[trimmed] {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func mergeFieldMaps(common map[string]bool, specific []string) map[string]bool {
	result := make(map[string]bool, len(common)+len(specific))
	for key := range common {
		result[key] = true
	}
	for _, key := range specific {
		result[strings.ToLower(key)] = true
	}
	return result
}

func subtractCommonFieldKeys(source map[string]bool, common map[string]bool) []string {
	if len(source) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(source))
	for key := range source {
		if common[key] {
			continue
		}
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func getCommonFields() map[string]bool {
	manager := currentFieldsManager()
	return manager.cloneFieldSet(manager.commonFields)
}

func getRequestHeaderFields() map[string]bool {
	manager := currentFieldsManager()
	return manager.cloneFieldSet(manager.requestHeaderFields)
}

func getRequestBodyFields() map[string]bool {
	manager := currentFieldsManager()
	fields := manager.cloneFieldSet(manager.requestBodyFields)
	immutableRequestBodyFieldsMu.RLock()
	defer immutableRequestBodyFieldsMu.RUnlock()
	for key := range immutableRequestBodyFields {
		fields[key] = true
	}
	return fields
}

func getResponseHeaderFields() map[string]bool {
	manager := currentFieldsManager()
	return manager.cloneFieldSet(manager.responseHeaderFields)
}

func getResponseBodyFields() map[string]bool {
	manager := currentFieldsManager()
	return manager.cloneFieldSet(manager.responseBodyFields)
}

// Credential names match whole keys or delimited parts, so removing a business
// mask still works for unrelated fields such as shipping, compass or tokenizer.
func isImmutableCredentialField(field string) bool {
	immutableRequestBodyFieldsMu.RLock()
	defer immutableRequestBodyFieldsMu.RUnlock()
	if immutableCredentialFields[field] {
		return true
	}
	for _, part := range strings.FieldsFunc(field, func(r rune) bool { return r == '_' || r == '-' || r == '.' }) {
		if immutableCredentialFields[part] {
			return true
		}
	}
	return false
}

func (m *sensitiveFieldsManager) cloneFieldSet(source map[string]bool) map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]bool, len(source))
	for k, v := range source {
		result[k] = v
	}
	return result
}

func mapKeys(source map[string]bool) []string {
	if len(source) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(source))
	for key := range source {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
