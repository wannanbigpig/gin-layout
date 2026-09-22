package sys_config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

const (
	TaskCronDemoEnabledConfigKey  = "task.cron_demo_enabled"
	TodoReminderEnabledConfigKey  = "todo.reminder_enabled"
	TodoReminderSMSEnabledKey     = "todo.reminder_sms_enabled"
	TodoReminderLeadMinutesKey    = "todo.reminder_lead_minutes"
	TodoReminderCatchupMinutesKey = "todo.reminder_catchup_minutes"
	TodoReminderMaxMinutes        = 30 * 24 * 60

	AuthLoginLockEnabledConfigKey = "auth.login_lock_enabled"
	AuthLoginMaxFailuresConfigKey = "auth.login_max_failures"
	AuthLoginLockMinutesConfigKey = "auth.login_lock_minutes"
)

// BoolValue 读取 bool 类型系统参数；读取失败或解析失败时返回 fallback。
func BoolValue(key string, fallback bool) bool {
	item, err := NewSysConfigService().Value(key)
	if err != nil {
		return fallback
	}
	value, err := strconv.ParseBool(item.ConfigValue)
	if err != nil {
		return fallback
	}
	return value
}

// IntValue 读取 int 类型系统参数；读取失败或解析失败时返回 fallback。
func IntValue(key string, fallback int) int {
	item, err := NewSysConfigService().Value(key)
	if err != nil {
		return fallback
	}
	value, err := strconv.Atoi(item.ConfigValue)
	if err != nil {
		return fallback
	}
	return value
}

// IntValueStrict 读取必须为整数的系统参数。配置缺失时使用 fallback，
// 已存在但格式错误时返回错误，避免运行时悄悄改变任务行为。
func IntValueStrict(key string, fallback int) (int, error) {
	return intValueStrict(key, fallback, NewSysConfigService().Value)
}

func intValueStrict(key string, fallback int, load func(string) (ConfigCacheItem, error)) (int, error) {
	item, err := load(key)
	if err != nil {
		var businessErr *e.BusinessError
		if errors.Is(err, gorm.ErrRecordNotFound) || (errors.As(err, &businessErr) && businessErr.GetCode() == e.NotFound) {
			return fallback, nil
		}
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(item.ConfigValue))
	if err != nil {
		return 0, fmt.Errorf("系统参数 %s 必须为整数: %w", key, err)
	}
	return value, nil
}
