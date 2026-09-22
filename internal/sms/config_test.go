package sms

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
)

type recordingProvider struct {
	input SendInput
}

func (p *recordingProvider) Send(_ context.Context, input SendInput) (SendResult, error) {
	p.input = input
	return SendResult{State: SendStateAccepted, ProviderMessageID: "message-1"}, nil
}

func TestDispatcherRoutesSceneThroughRegisteredProvider(t *testing.T) {
	provider := &recordingProvider{}
	registry := NewRegistry()
	if err := registry.Register(ProviderDefinition{Name: "custom", Factory: func(_ json.RawMessage) (Provider, error) { return provider, nil }}); err != nil {
		t.Fatalf("register custom provider: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Providers["custom"] = json.RawMessage(`{"enabled":true}`)
	cfg.Templates["custom"] = map[string]Template{
		"login": {SignName: "Gin Layout", TemplateCode: "TPL_LOGIN", Variables: []string{"code"}},
	}
	cfg.Routes[SceneLoginOTP] = Route{Provider: "custom", Template: "login"}

	dispatcher, err := NewDispatcher(cfg, registry)
	if err != nil {
		t.Fatalf("build dispatcher: %v", err)
	}
	result, err := dispatcher.Send(context.Background(), SceneLoginOTP, "+8613800000000", map[string]string{"code": "123456"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if result.State != SendStateAccepted || provider.input.TemplateCode != "TPL_LOGIN" || provider.input.Variables["code"] != "123456" {
		t.Fatalf("unexpected routed message: result=%#v input=%#v", result, provider.input)
	}
}

func TestRegistryRejectsDuplicateProvider(t *testing.T) {
	registry := NewRegistry()
	definition := ProviderDefinition{Name: "custom", Factory: func(_ json.RawMessage) (Provider, error) { return &recordingProvider{}, nil }}
	if err := registry.Register(definition); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if err := registry.Register(definition); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Fatalf("expected duplicate registration error, got %v", err)
	}
}

func TestRegistrySensitiveFieldsExtendImmutableRequestMask(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(ProviderDefinition{
		Name:            "custom-mask",
		SensitiveFields: []string{"provider_credential"},
		Factory:         func(_ json.RawMessage) (Provider, error) { return &recordingProvider{}, nil },
	}); err != nil {
		t.Fatal(err)
	}
	masked := sensitive.GetMaskedRequestBody([]byte(`{"providers":{"custom-mask":{"provider_credential":"must-not-leak"}}}`), "application/json")
	if strings.Contains(masked, "must-not-leak") {
		t.Fatalf("registered provider credential leaked from request mask: %s", masked)
	}
}

func TestConfigValidationUsesRegisteredProviderFactory(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(ProviderDefinition{
		Name: "custom",
		Factory: func(raw json.RawMessage) (Provider, error) {
			if !strings.Contains(string(raw), `"enabled":true`) {
				return nil, errors.New("custom provider disabled")
			}
			return &recordingProvider{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Providers["custom"] = json.RawMessage(`{"enabled":false}`)
	cfg.Templates["custom"] = map[string]Template{"login": {SignName: "Gin Layout", TemplateCode: "TPL_LOGIN", Variables: []string{"code"}}}
	cfg.Routes[SceneLoginOTP] = Route{Provider: "custom", Template: "login"}
	if err := cfg.ValidateWithRegistry(registry); err == nil || !strings.Contains(err.Error(), "custom provider disabled") {
		t.Fatalf("expected factory validation error, got %v", err)
	}
}

func TestConfigRejectsUnknownSceneAndBrokenRoute(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Routes["dynamic_scene"] = Route{Provider: ProviderAliyun, Template: "login_otp"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected unknown scene to be rejected")
	}

	cfg = DefaultConfig()
	cfg.Enabled = true
	cfg.Providers[ProviderAliyun] = json.RawMessage(`{"enabled":true,"access_key_id":"id","access_key_secret":"secret"}`)
	cfg.Routes[SceneLoginOTP] = Route{Provider: ProviderAliyun, Template: "missing"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing template to be rejected")
	}
}

func TestConfigValidatesTodoDueTemplateVariables(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Providers[ProviderAliyun] = json.RawMessage(`{"enabled":true,"access_key_id":"id","access_key_secret":"secret"}`)
	login := cfg.Templates[ProviderAliyun]["login_otp"]
	login.SignName = "Gin Layout"
	login.TemplateCode = "TPL_LOGIN"
	cfg.Templates[ProviderAliyun]["login_otp"] = login
	todo := cfg.Templates[ProviderAliyun]["todo_due"]
	todo.SignName = "Gin Layout"
	todo.TemplateCode = "TPL_TODO"
	cfg.Templates[ProviderAliyun]["todo_due"] = todo
	cfg.Routes[SceneTodoDue] = Route{Provider: ProviderAliyun, Template: "todo_due"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid todo_due route: %v", err)
	}

	todo.Variables = []string{"title", "due_at", "phone"}
	cfg.Templates[ProviderAliyun]["todo_due"] = todo
	if err := cfg.Validate(); err == nil {
		t.Fatal("todo_due template must reject variables outside title and due_at")
	}
}

func TestConfigRejectsRegionIDAsAliyunEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Providers[ProviderAliyun] = json.RawMessage(`{"enabled":true,"access_key_id":"id","access_key_secret":"secret","endpoint":"cn-hangzhou"}`)
	login := cfg.Templates[ProviderAliyun]["login_otp"]
	login.SignName = "Gin Layout"
	login.TemplateCode = "TPL_LOGIN"
	cfg.Templates[ProviderAliyun]["login_otp"] = login
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "完整官方 URL") || !strings.Contains(err.Error(), "不能填写 Region ID") {
		t.Fatalf("expected actionable endpoint validation, got %v", err)
	}
}
