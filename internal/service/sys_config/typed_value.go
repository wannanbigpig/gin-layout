package sys_config

import "strconv"

const (
	TaskCronDemoEnabledConfigKey = "task.cron_demo_enabled"

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
