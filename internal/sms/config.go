package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
)

const (
	ProviderAliyun  = "aliyun"
	SceneLoginOTP   = "login_otp"
	SceneTodoDue    = "todo_due"
	MaskPlaceholder = "********"
)

var allowedScenes = map[string]struct{}{
	SceneLoginOTP: {},
	SceneTodoDue:  {},
}

type Config struct {
	Enabled   bool                           `json:"enabled"`
	Providers map[string]json.RawMessage     `json:"providers"`
	Templates map[string]map[string]Template `json:"templates"`
	Routes    map[string]Route               `json:"routes"`
	OTP       OTPConfig                      `json:"otp"`
}

type OTPConfig struct {
	TTLSeconds            int `json:"ttl_seconds"`
	ResendIntervalSeconds int `json:"resend_interval_seconds"`
	MaxAttempts           int `json:"max_attempts"`
	IPLimitPer10Minutes   int `json:"ip_limit_per_10_minutes"`
	PhoneDailyLimit       int `json:"phone_daily_limit"`
	GlobalDailyLimit      int `json:"global_daily_limit"`
}

type Template struct {
	SignName     string   `json:"sign_name"`
	TemplateCode string   `json:"template_code"`
	Variables    []string `json:"variables"`
}

type Route struct {
	Provider string `json:"provider"`
	Template string `json:"template"`
}

type AliyunConfig struct {
	Enabled         bool   `json:"enabled"`
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	Endpoint        string `json:"endpoint"`
}

func DefaultConfig() Config {
	aliyun, _ := json.Marshal(AliyunConfig{Endpoint: "https://dysmsapi.aliyuncs.com"})
	return Config{
		Enabled:   false,
		Providers: map[string]json.RawMessage{ProviderAliyun: aliyun},
		Templates: map[string]map[string]Template{
			ProviderAliyun: {
				"login_otp": {Variables: []string{"code"}},
				"todo_due":  {Variables: []string{"title", "due_at"}},
			},
		},
		Routes: map[string]Route{
			SceneLoginOTP: {Provider: ProviderAliyun, Template: "login_otp"},
		},
		OTP: OTPConfig{
			TTLSeconds:            300,
			ResendIntervalSeconds: 60,
			MaxAttempts:           5,
			IPLimitPer10Minutes:   10,
			PhoneDailyLimit:       20,
			GlobalDailyLimit:      1000,
		},
	}
}

func (c *Config) ApplyDefaults() {
	defaults := DefaultConfig()
	if c.Providers == nil {
		c.Providers = defaults.Providers
	}
	if c.Templates == nil {
		c.Templates = defaults.Templates
	}
	if c.Routes == nil {
		c.Routes = defaults.Routes
	}
	if c.OTP.TTLSeconds <= 0 {
		c.OTP.TTLSeconds = defaults.OTP.TTLSeconds
	}
	if c.OTP.ResendIntervalSeconds <= 0 {
		c.OTP.ResendIntervalSeconds = defaults.OTP.ResendIntervalSeconds
	}
	if c.OTP.MaxAttempts <= 0 {
		c.OTP.MaxAttempts = defaults.OTP.MaxAttempts
	}
	if c.OTP.IPLimitPer10Minutes <= 0 {
		c.OTP.IPLimitPer10Minutes = defaults.OTP.IPLimitPer10Minutes
	}
	if c.OTP.PhoneDailyLimit <= 0 {
		c.OTP.PhoneDailyLimit = defaults.OTP.PhoneDailyLimit
	}
	if c.OTP.GlobalDailyLimit <= 0 {
		c.OTP.GlobalDailyLimit = defaults.OTP.GlobalDailyLimit
	}
}

func (c Config) Validate() error {
	return c.ValidateWithRegistry(DefaultRegistry())
}

// ValidateWithRegistry validates provider-independent scene/template rules and
// delegates provider-specific configuration validation to the registered
// factory. This keeps service and business modules free of provider branches.
func (c Config) ValidateWithRegistry(registry *Registry) error {
	if !c.Enabled {
		return nil
	}
	if c.OTP.TTLSeconds < 60 || c.OTP.TTLSeconds > 900 {
		return errors.New("短信验证码有效期必须在 60 到 900 秒之间")
	}
	if c.OTP.ResendIntervalSeconds < 30 || c.OTP.ResendIntervalSeconds > c.OTP.TTLSeconds {
		return errors.New("短信重发间隔必须在 30 秒到验证码有效期之间")
	}
	if c.OTP.MaxAttempts < 1 || c.OTP.MaxAttempts > 10 {
		return errors.New("短信验证码最大尝试次数必须在 1 到 10 次之间")
	}
	if c.OTP.IPLimitPer10Minutes < 1 || c.OTP.PhoneDailyLimit < 1 || c.OTP.GlobalDailyLimit < 1 {
		return errors.New("短信发送限额必须大于 0")
	}
	if registry == nil {
		return errors.New("短信供应商注册表未初始化")
	}
	validatedProviders := make(map[string]struct{})
	for scene, route := range c.Routes {
		if _, ok := allowedScenes[scene]; !ok {
			return fmt.Errorf("不支持的短信场景: %s", scene)
		}
		providerName := strings.TrimSpace(route.Provider)
		templateName := strings.TrimSpace(route.Template)
		if providerName == "" || templateName == "" {
			return fmt.Errorf("短信场景 %s 的供应商和模板不能为空", scene)
		}
		if _, ok := c.Providers[providerName]; !ok {
			return fmt.Errorf("短信场景 %s 引用了未配置的供应商 %s", scene, providerName)
		}
		if _, validated := validatedProviders[providerName]; !validated {
			if _, err := registry.Build(providerName, c.Providers[providerName]); err != nil {
				return fmt.Errorf("短信场景 %s 引用的供应商 %s 配置无效: %w", scene, providerName, err)
			}
			validatedProviders[providerName] = struct{}{}
		}
		providerTemplates, ok := c.Templates[providerName]
		if !ok {
			return fmt.Errorf("短信供应商 %s 未配置模板", providerName)
		}
		template, ok := providerTemplates[templateName]
		if !ok {
			return fmt.Errorf("短信场景 %s 引用了不存在的模板 %s", scene, templateName)
		}
		if strings.TrimSpace(template.SignName) == "" || strings.TrimSpace(template.TemplateCode) == "" {
			return fmt.Errorf("短信模板 %s/%s 的签名和模板编码不能为空", providerName, templateName)
		}
		if scene == SceneLoginOTP && !sameStringSet(template.Variables, []string{"code"}) {
			return errors.New("登录验证码模板变量必须且只能包含 code")
		}
		if scene == SceneTodoDue && !sameStringSet(template.Variables, []string{"title", "due_at"}) {
			return errors.New("待办到期模板变量必须且只能包含 title、due_at")
		}
	}
	if _, ok := c.Routes[SceneLoginOTP]; !ok {
		return errors.New("启用短信后必须配置登录验证码场景")
	}
	return nil
}

type SendState string

const (
	// SendStateAccepted means the provider API accepted the request; it is not a handset delivery receipt.
	SendStateAccepted SendState = "accepted"
	// SendStateFailed means the provider definitely rejected the request and a caller may apply its retry policy.
	SendStateFailed SendState = "failed"
	// SendStateUnknown means transport interruption or timeout left the outcome uncertain; callers must not blindly retry.
	SendStateUnknown SendState = "unknown"
)

type SendInput struct {
	PhoneNumber  string
	SignName     string
	TemplateCode string
	Variables    map[string]string
}

type SendResult struct {
	State             SendState
	ProviderRequestID string
	ProviderMessageID string
	ProviderCode      string
}

type Provider interface {
	Send(ctx context.Context, input SendInput) (SendResult, error)
}

type ProviderFactory func(config json.RawMessage) (Provider, error)

// ProviderDefinition describes one compile-time SMS provider integration.
// SensitiveFields only supports top-level string fields in the provider JSON;
// SMSConfigService uses it to merge, encrypt, mask, and audit credentials
// without adding provider-specific branches.
type ProviderDefinition struct {
	Name            string
	Factory         ProviderFactory
	SensitiveFields []string
}

type Registry struct {
	mu          sync.RWMutex
	definitions map[string]ProviderDefinition
}

var builtinProviderDefinitions = []ProviderDefinition{
	{
		Name:            ProviderAliyun,
		Factory:         NewAliyunProvider,
		SensitiveFields: []string{"access_key_secret"},
	},
}

func init() {
	// Request bodies are cached before controller binding. Register the immutable
	// masking baseline at package initialization so even a rejected config
	// request cannot log provider credentials.
	for _, definition := range builtinProviderDefinitions {
		sensitive.RegisterImmutableRequestBodyFields(definition.SensitiveFields...)
	}
}

func NewRegistry() *Registry {
	return &Registry{definitions: make(map[string]ProviderDefinition)}
}

func DefaultRegistry() *Registry {
	r := NewRegistry()
	if err := RegisterBuiltinProviders(r); err != nil {
		panic(err)
	}
	return r
}

// RegisterBuiltinProviders is the single compile-time registration entrypoint.
// A second-party provider should add one definition here; business services do
// not need to know the provider name.
func RegisterBuiltinProviders(r *Registry) error {
	if r == nil {
		return errors.New("短信供应商注册表未初始化")
	}
	for _, definition := range builtinProviderDefinitions {
		if err := r.Register(definition); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) Register(definition ProviderDefinition) error {
	if r == nil {
		return errors.New("短信供应商注册表未初始化")
	}
	name := strings.TrimSpace(definition.Name)
	if name == "" || definition.Factory == nil {
		return errors.New("短信供应商名称和 factory 不能为空")
	}
	definition.Name = name
	definition.SensitiveFields = normalizeProviderSensitiveFields(definition.SensitiveFields)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.definitions[name]; exists {
		return fmt.Errorf("短信供应商 %s 已注册", name)
	}
	r.definitions[name] = definition
	sensitive.RegisterImmutableRequestBodyFields(definition.SensitiveFields...)
	return nil
}

func (r *Registry) Definition(name string) (ProviderDefinition, bool) {
	if r == nil {
		return ProviderDefinition{}, false
	}
	r.mu.RLock()
	definition, ok := r.definitions[strings.TrimSpace(name)]
	r.mu.RUnlock()
	if !ok {
		return ProviderDefinition{}, false
	}
	definition.SensitiveFields = append([]string(nil), definition.SensitiveFields...)
	return definition, true
}

func (r *Registry) Build(name string, config json.RawMessage) (Provider, error) {
	if r == nil {
		return nil, errors.New("短信供应商注册表未初始化")
	}
	definition, ok := r.Definition(name)
	if !ok {
		return nil, fmt.Errorf("短信供应商 %s 未注册", name)
	}
	return definition.Factory(config)
}

func normalizeProviderSensitiveFields(fields []string) []string {
	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		result = append(result, field)
	}
	return result
}

type Dispatcher struct {
	config    Config
	providers map[string]Provider
}

func NewDispatcher(config Config, registry *Registry) (*Dispatcher, error) {
	config.ApplyDefaults()
	if registry == nil {
		registry = DefaultRegistry()
	}
	if err := config.ValidateWithRegistry(registry); err != nil {
		return nil, err
	}
	providers := make(map[string]Provider)
	for _, route := range config.Routes {
		if _, ok := providers[route.Provider]; ok {
			continue
		}
		provider, err := registry.Build(route.Provider, config.Providers[route.Provider])
		if err != nil {
			return nil, err
		}
		providers[route.Provider] = provider
	}
	return &Dispatcher{config: config, providers: providers}, nil
}

func (d *Dispatcher) Send(ctx context.Context, scene, phone string, variables map[string]string) (SendResult, error) {
	if d == nil || !d.config.Enabled {
		return SendResult{}, errors.New("短信服务未启用")
	}
	if _, ok := allowedScenes[scene]; !ok {
		return SendResult{}, fmt.Errorf("不支持的短信场景: %s", scene)
	}
	route, ok := d.config.Routes[scene]
	if !ok {
		return SendResult{}, fmt.Errorf("短信场景 %s 未配置路由", scene)
	}
	template, ok := d.config.Templates[route.Provider][route.Template]
	if !ok {
		return SendResult{}, fmt.Errorf("短信场景 %s 的模板不存在", scene)
	}
	provider := d.providers[route.Provider]
	if provider == nil {
		return SendResult{}, fmt.Errorf("短信供应商 %s 未初始化", route.Provider)
	}
	if !sameStringSet(template.Variables, mapKeys(variables)) {
		return SendResult{}, fmt.Errorf("短信场景 %s 的模板变量不匹配", scene)
	}
	return provider.Send(ctx, SendInput{
		PhoneNumber: phone, SignName: template.SignName, TemplateCode: template.TemplateCode, Variables: variables,
	})
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	set := make(map[string]struct{}, len(left))
	for _, value := range left {
		value = strings.TrimSpace(value)
		if value == "" {
			return false
		}
		set[value] = struct{}{}
	}
	if len(set) != len(left) {
		return false
	}
	for _, value := range right {
		if _, ok := set[strings.TrimSpace(value)]; !ok {
			return false
		}
	}
	return true
}
