package sensitive

import (
	"regexp"
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
	m.commonFields = sliceToMap(config.Common)
	m.requestHeaderFields = sliceToMap(config.RequestHeader)
	m.requestBodyFields = sliceToMap(config.RequestBody)
	m.responseHeaderFields = sliceToMap(config.ResponseHeader)
	m.responseBodyFields = sliceToMap(config.ResponseBody)
}

func defaultSensitiveFieldsConfig() SensitiveFieldsConfig {
	return SensitiveFieldsConfig{
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
}

// LoadSensitiveFieldsConfig 加载敏感字段配置（未来可从配置文件调用）
func LoadSensitiveFieldsConfig(config SensitiveFieldsConfig) {
	manager := currentFieldsManager()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.applyConfig(config)
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
	return manager.cloneFieldSet(manager.requestBodyFields)
}

func getResponseHeaderFields() map[string]bool {
	manager := currentFieldsManager()
	return manager.cloneFieldSet(manager.responseHeaderFields)
}

func getResponseBodyFields() map[string]bool {
	manager := currentFieldsManager()
	return manager.cloneFieldSet(manager.responseBodyFields)
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
