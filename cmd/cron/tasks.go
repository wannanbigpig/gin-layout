package cron

import (
	"time"

	"go.uber.org/zap"

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

	schedule.CallE("reset-system-data", system.ReinitializeSystemData).
		DailyAt("02:00:00").
		WithoutOverlapping()
}
