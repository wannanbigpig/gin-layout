package sys_config

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestBoolValueFallbackWhenMissing(t *testing.T) {
	if got := BoolValue("missing.key", true); !got {
		t.Fatal("expected fallback true when config is missing")
	}
	if got := BoolValue("missing.key", false); got {
		t.Fatal("expected fallback false when config is missing")
	}
}

func TestBoolValueFromCache(t *testing.T) {
	restore := setRuntimeCacheForTest([]model.SysConfig{
		{ConfigKey: AuthLoginLockEnabledConfigKey, ConfigValue: "true", Status: 1},
	})
	t.Cleanup(restore)

	if !BoolValue(AuthLoginLockEnabledConfigKey, false) {
		t.Fatal("expected bool value from cache")
	}
}

func TestIntValueFromCache(t *testing.T) {
	restore := setRuntimeCacheForTest([]model.SysConfig{
		{ConfigKey: AuthLoginMaxFailuresConfigKey, ConfigValue: "7", Status: 1},
	})
	t.Cleanup(restore)

	if got := IntValue(AuthLoginMaxFailuresConfigKey, 5); got != 7 {
		t.Fatalf("expected int value 7, got %d", got)
	}
}

func TestIntValueFallbackWhenInvalid(t *testing.T) {
	restore := setRuntimeCacheForTest([]model.SysConfig{
		{ConfigKey: AuthLoginLockMinutesConfigKey, ConfigValue: "invalid", Status: 1},
	})
	t.Cleanup(restore)

	if got := IntValue(AuthLoginLockMinutesConfigKey, 15); got != 15 {
		t.Fatalf("expected fallback int value 15, got %d", got)
	}
}

func setRuntimeCacheForTest(configs []model.SysConfig) func() {
	runtimeCache.Lock()
	prevLoaded := runtimeCache.loaded
	prevItems := runtimeCache.items
	runtimeCache.Unlock()

	replaceCache(configs)

	return func() {
		runtimeCache.Lock()
		runtimeCache.loaded = prevLoaded
		runtimeCache.items = prevItems
		runtimeCache.Unlock()
	}
}
