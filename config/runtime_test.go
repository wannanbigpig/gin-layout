package config

import (
	"testing"

	. "github.com/wannanbigpig/gin-layout/config/autoload"
)

func TestBuildConfigDiff(t *testing.T) {
	oldCfg := &Conf{
		AppConfig: App,
		Mysql:     Mysql,
		Redis:     Redis,
		Logger:    Logger,
		Jwt:       Jwt,
	}
	newCfg := &Conf{
		AppConfig: App,
		Mysql:     Mysql,
		Redis:     Redis,
		Logger:    Logger,
		Jwt:       Jwt,
	}

	newCfg.Logger.Output = "stderr"
	newCfg.Redis.Enable = true
	newCfg.BaseURL = "https://example.com"
	newCfg.CorsOrigins = []string{"https://ui.example.com"}
	newCfg.TrustedProxies = []string{"10.0.0.0/8"}
	newCfg.Jwt.TTL = 999
	newCfg.Jwt.SecretKey = "new-secret"
	newCfg.Language = "en"

	diff := BuildConfigDiff(oldCfg, newCfg)
	if !diff.LoggerChanged {
		t.Fatalf("expected logger change")
	}
	if !diff.RedisChanged {
		t.Fatalf("expected redis change")
	}
	if !diff.JWTChanged {
		t.Fatalf("expected jwt ttl change")
	}
	if !diff.JWTSecretChanged {
		t.Fatalf("expected jwt secret change")
	}
	if !diff.BaseURLChanged {
		t.Fatalf("expected base_url change")
	}
	if !diff.CORSChanged {
		t.Fatalf("expected cors change")
	}
	if !diff.TrustedProxiesChanged {
		t.Fatalf("expected trusted proxies change")
	}
	if len(diff.RestartRequiredFields) == 0 {
		t.Fatalf("expected restart-required fields")
	}
}

func TestBuildAppliedConfigKeepsUnsupportedFields(t *testing.T) {
	oldCfg := &Conf{
		AppConfig: App,
		Mysql:     Mysql,
		Redis:     Redis,
		Logger:    Logger,
		Jwt: JwtConfig{
			TTL:        100,
			RefreshTTL: 10,
			SecretKey:  "old-secret",
		},
	}
	oldCfg.TrustedProxies = []string{"127.0.0.1"}
	oldCfg.Language = "zh_CN"

	newCfg := &Conf{
		AppConfig: App,
		Mysql:     Mysql,
		Redis:     Redis,
		Logger:    Logger,
		Jwt: JwtConfig{
			TTL:        200,
			RefreshTTL: 20,
			SecretKey:  "new-secret",
		},
	}
	newCfg.TrustedProxies = []string{"10.0.0.0/8"}
	newCfg.Language = "en"
	newCfg.BaseURL = "https://cdn.example.com"

	diff := BuildConfigDiff(oldCfg, newCfg)
	applied := BuildAppliedConfig(oldCfg, newCfg, diff)

	if applied.Jwt.SecretKey != "old-secret" {
		t.Fatalf("expected jwt secret to remain old, got %q", applied.Jwt.SecretKey)
	}
	if applied.Language != "zh_CN" {
		t.Fatalf("expected language to remain old, got %q", applied.Language)
	}
	if len(applied.TrustedProxies) != 1 || applied.TrustedProxies[0] != "127.0.0.1" {
		t.Fatalf("expected trusted proxies to remain old, got %#v", applied.TrustedProxies)
	}
	if applied.BaseURL != "https://cdn.example.com" {
		t.Fatalf("expected supported field base_url to update, got %q", applied.BaseURL)
	}
	if applied.Jwt.TTL != 200 {
		t.Fatalf("expected supported jwt ttl to update, got %v", applied.Jwt.TTL)
	}
}
