package sys_config

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
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

func TestIntValueStrictRejectsInvalidConfiguredValue(t *testing.T) {
	restore := setRuntimeCacheForTest([]model.SysConfig{
		{ConfigKey: TodoReminderLeadMinutesKey, ConfigValue: "1.5", Status: 1},
	})
	t.Cleanup(restore)

	if _, err := IntValueStrict(TodoReminderLeadMinutesKey, 60); err == nil {
		t.Fatal("expected decimal reminder minutes to be rejected")
	}
}

func TestIntValueStrictFallsBackOnlyWhenConfigIsMissing(t *testing.T) {
	for _, missingErr := range []error{gorm.ErrRecordNotFound, e.NewBusinessError(e.NotFound)} {
		value, err := intValueStrict("missing.key", 60, func(string) (ConfigCacheItem, error) {
			return ConfigCacheItem{}, missingErr
		})
		if err != nil || value != 60 {
			t.Fatalf("expected missing config fallback, value=%d err=%v", value, err)
		}
	}
}

func TestIntValueStrictPreservesTechnicalFailure(t *testing.T) {
	technicalErr := errors.New("database unavailable")
	_, err := intValueStrict(TodoReminderLeadMinutesKey, 60, func(string) (ConfigCacheItem, error) {
		return ConfigCacheItem{}, technicalErr
	})
	if !errors.Is(err, technicalErr) {
		t.Fatalf("expected technical error to be preserved, got %v", err)
	}
}

func TestValidateKnownConfigValue(t *testing.T) {
	for _, value := range []string{"0", "1440", "43200"} {
		if err := validateKnownConfigValue(TodoReminderLeadMinutesKey, value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}
	for _, value := range []string{"1.5", "-1", "43201", "invalid"} {
		if err := validateKnownConfigValue(TodoReminderLeadMinutesKey, value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
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
