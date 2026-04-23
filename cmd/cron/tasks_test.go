package cron

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
)

func TestDefineScheduleSkipsResetTaskByDefault(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.EnableResetSystemCron = false
	})
	defer restoreConfig()

	schedule := NewSchedule(&cronLogger{})
	defineSchedule(schedule)

	if hasScheduledTask(schedule, "reset-system-data") {
		t.Fatal("expected reset-system-data task to be skipped by default")
	}
	if !hasScheduledTask(schedule, "demo") {
		t.Fatal("expected demo task to remain registered")
	}
}

func TestDefineScheduleRegistersResetTaskWhenEnabled(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.EnableResetSystemCron = true
	})
	defer restoreConfig()

	schedule := NewSchedule(&cronLogger{})
	defineSchedule(schedule)

	if !hasScheduledTask(schedule, "reset-system-data") {
		t.Fatal("expected reset-system-data task to be registered when enabled")
	}
}

func hasScheduledTask(schedule *Scheduler, name string) bool {
	for _, task := range schedule.tasks {
		if task.name == name {
			return true
		}
	}
	return false
}
