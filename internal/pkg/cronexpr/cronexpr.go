package cronexpr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/robfig/cron/v3"
)

var parser = cron.NewParser(
	cron.Second |
		cron.Minute |
		cron.Hour |
		cron.Dom |
		cron.Month |
		cron.Dow |
		cron.Descriptor,
)

// Validate checks whether spec is a valid 6-field cron expression or descriptor.
func Validate(spec string) error {
	_, err := Parse(spec)
	return err
}

// Parse parses spec with the same parser used by the cron process.
func Parse(spec string) (cron.Schedule, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("cron 表达式不能为空")
	}
	return parser.Parse(spec)
}

// Describe returns a concise Chinese description for common cron expressions.
func Describe(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	if text := describeDescriptor(spec); text != "" {
		return text
	}
	fields := strings.Fields(spec)
	if len(fields) != 6 {
		return "自定义调度：" + spec
	}

	second, minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4], fields[5]
	if dom == "*" && month == "*" && dow == "*" {
		if text := describeEverySeconds(second, minute, hour); text != "" {
			return text
		}
		if text := describeEveryMinutes(second, minute, hour); text != "" {
			return text
		}
		if text := describeHourly(second, minute, hour); text != "" {
			return text
		}
		if text := describeDaily(second, minute, hour); text != "" {
			return text
		}
	}
	if month == "*" && isFixedNumber(second) && isFixedNumber(minute) && isFixedNumber(hour) {
		if dom != "*" && dow == "*" {
			return fmt.Sprintf("每月 %s 日 %s 执行", dom, formatTime(hour, minute, second, true))
		}
		if dom == "*" && dow != "*" {
			return fmt.Sprintf("每周 %s %s 执行", describeWeekday(dow), formatTime(hour, minute, second, true))
		}
	}
	return "自定义调度：" + spec
}

func describeDescriptor(spec string) string {
	switch strings.ToLower(spec) {
	case "@yearly", "@annually":
		return "每年 1 月 1 日 00:00 执行"
	case "@monthly":
		return "每月 1 日 00:00 执行"
	case "@weekly":
		return "每周日 00:00 执行"
	case "@daily", "@midnight":
		return "每天 00:00 执行"
	case "@hourly":
		return "每小时整点执行"
	}
	return ""
}

func describeEverySeconds(second, minute, hour string) string {
	if minute != "*" || hour != "*" {
		return ""
	}
	if step, ok := parseEveryStep(second); ok {
		if step == 1 {
			return "每秒执行一次"
		}
		return fmt.Sprintf("每 %d 秒执行一次", step)
	}
	return ""
}

func describeEveryMinutes(second, minute, hour string) string {
	if second != "0" || hour != "*" {
		return ""
	}
	if step, ok := parseEveryStep(minute); ok {
		if step == 1 {
			return "每分钟执行一次"
		}
		return fmt.Sprintf("每 %d 分钟执行一次", step)
	}
	return ""
}

func describeHourly(second, minute, hour string) string {
	if hour != "*" || !isFixedNumber(second) || !isFixedNumber(minute) {
		return ""
	}
	if second == "0" && minute == "0" {
		return "每小时整点执行"
	}
	return fmt.Sprintf("每小时第 %s 分 %s 秒执行", minute, second)
}

func describeDaily(second, minute, hour string) string {
	if !isFixedNumber(second) || !isFixedNumber(minute) || !isFixedNumber(hour) {
		return ""
	}
	return fmt.Sprintf("每天 %s 执行", formatTime(hour, minute, second, true))
}

func parseEveryStep(value string) (int, bool) {
	if value == "*" {
		return 1, true
	}
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] != "*" && parts[0] != "0" {
		return 0, false
	}
	step, err := strconv.Atoi(parts[1])
	if err != nil || step <= 0 {
		return 0, false
	}
	return step, true
}

func isFixedNumber(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

func formatTime(hour, minute, second string, omitZeroSecond bool) string {
	if omitZeroSecond && atoi(second) == 0 {
		return fmt.Sprintf("%02d:%02d", atoi(hour), atoi(minute))
	}
	return fmt.Sprintf("%02d:%02d:%02d", atoi(hour), atoi(minute), atoi(second))
}

func describeWeekday(value string) string {
	labels := map[string]string{
		"0":   "日",
		"1":   "一",
		"2":   "二",
		"3":   "三",
		"4":   "四",
		"5":   "五",
		"6":   "六",
		"7":   "日",
		"SUN": "日",
		"MON": "一",
		"TUE": "二",
		"WED": "三",
		"THU": "四",
		"FRI": "五",
		"SAT": "六",
	}
	upper := strings.ToUpper(value)
	if label, ok := labels[upper]; ok {
		return "周" + label
	}
	return value
}

func atoi(value string) int {
	number, _ := strconv.Atoi(value)
	return number
}
