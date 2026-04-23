package cron

import (
	"time"

	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/system"
)

const timeFormat = "2006-01-02 15:04:05"

func defineSchedule(schedule *Scheduler) {
	schedule.Call("demo", func() {
		log.Logger.Info("计划任务 demo 执行：", zap.String("time", time.Now().Format(timeFormat)))
	}).
		EveryFiveSeconds().
		WithoutOverlapping()

	cfg := config.GetConfig()
	if cfg == nil || !cfg.EnableResetSystemCron {
		return
	}

	log.Logger.Warn("高风险定时任务已启用",
		zap.String("name", "reset-system-data"),
		zap.String("schedule", "02:00:00"),
	)
	schedule.CallE("reset-system-data", system.ReinitializeSystemData).
		DailyAt("02:00:00").
		WithoutOverlapping()
}
