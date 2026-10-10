package taskcron

import (
	"context"
	"fmt"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/service/sys_config"
	"github.com/wannanbigpig/gin-layout/internal/service/todo"
)

const (
	defaultTodoReminderCatchupMinutes = 7 * 24 * 60
)

// TodoReminderHandler 扫描临期和最近逾期的待办，按系统与用户渠道偏好生成站内通知和短信 Outbox。
// 当前部署约束仍是单 cron 实例；业务唯一键只负责重复扫描防重，不替代多调度器协调。
func TodoReminderHandler(ctx context.Context, _ map[string]any) error {
	if !sys_config.BoolValue(sys_config.TodoReminderEnabledConfigKey, true) {
		return nil
	}
	catchupMinutes, err := reminderMinutes(sys_config.TodoReminderCatchupMinutesKey, defaultTodoReminderCatchupMinutes)
	if err != nil {
		return err
	}
	_, err = todo.NewReminderService().Run(ctx, todo.ReminderOptions{
		LeadTime:      time.Duration(todo.MaxReminderLeadMinutes) * time.Minute,
		CatchupWindow: time.Duration(catchupMinutes) * time.Minute,
		SMSEnabled:    sys_config.BoolValue(sys_config.TodoReminderSMSEnabledKey, false),
	})
	return err
}

func reminderMinutes(key string, fallback int) (int, error) {
	value, err := sys_config.IntValueStrict(key, fallback)
	if err != nil {
		return 0, err
	}
	if value < 0 || value > sys_config.TodoReminderMaxMinutes {
		return 0, fmt.Errorf("系统参数 %s 必须在 0-%d 分钟之间", key, sys_config.TodoReminderMaxMinutes)
	}
	return value, nil
}
