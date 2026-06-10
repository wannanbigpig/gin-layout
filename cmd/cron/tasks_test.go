package cron

import (
	"context"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestDefineScheduleRegistersDefaultDisabledTasksForRuntimeSwitch(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.EnableResetSystemCron = false
	})
	defer restoreConfig()
	restoreStatus := setCronTaskStatusForTest(nil)
	defer restoreStatus()

	schedule := NewSchedule(&cronLogger{})
	defineSchedule(schedule)

	if !hasScheduledTask(schedule, "reset-system-data") {
		t.Fatal("expected reset-system-data task to be registered for runtime switch")
	}
	if !hasScheduledTask(schedule, "demo") {
		t.Fatal("expected demo task to be registered for runtime switch")
	}
	if taskEnabled(t, schedule, "demo") {
		t.Fatal("expected demo task to be disabled by default")
	}
	if taskEnabled(t, schedule, "reset-system-data") {
		t.Fatal("expected reset-system-data task to be disabled by default")
	}
}

func TestDefineScheduleRegistersResetTaskWhenEnabled(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.EnableResetSystemCron = true
	})
	defer restoreConfig()
	restoreStatus := setCronTaskStatusForTest(nil)
	defer restoreStatus()

	schedule := NewSchedule(&cronLogger{})
	defineSchedule(schedule)

	if !hasScheduledTask(schedule, "reset-system-data") {
		t.Fatal("expected reset-system-data task to be registered when enabled")
	}
	if !taskEnabled(t, schedule, "reset-system-data") {
		t.Fatal("expected reset-system-data task to be enabled when config enables it")
	}
}

func TestDefineScheduleUsesDBStatusToEnableTaskWithoutRestart(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.EnableResetSystemCron = false
	})
	defer restoreConfig()
	restoreStatus := setCronTaskStatusForTest(map[string]uint8{
		"cron:demo": model.TaskStatusEnabled,
	})
	defer restoreStatus()

	schedule := NewSchedule(&cronLogger{})
	defineSchedule(schedule)

	if !hasScheduledTask(schedule, "demo") {
		t.Fatal("expected demo task to be registered")
	}
	if !taskEnabled(t, schedule, "demo") {
		t.Fatal("expected demo task to be enabled when DB status enables it")
	}
}

func TestDefineScheduleUsesDBStatusToDisableTaskWithoutRestart(t *testing.T) {
	restoreStatus := setCronTaskStatusForTest(map[string]uint8{
		"cron:file-access-flush": model.TaskStatusDisabled,
	})
	defer restoreStatus()

	schedule := NewSchedule(&cronLogger{})
	defineSchedule(schedule)

	if !hasScheduledTask(schedule, "file-access-flush") {
		t.Fatal("expected file-access-flush task to be registered")
	}
	if taskEnabled(t, schedule, "file-access-flush") {
		t.Fatal("expected file-access-flush task to be disabled when DB status disables it")
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

func taskEnabled(t *testing.T, schedule *Scheduler, name string) bool {
	t.Helper()
	for _, task := range schedule.tasks {
		if task.name != name {
			continue
		}
		if task.enabledWhen == nil {
			return true
		}
		return task.enabledWhen(context.Background())
	}
	t.Fatalf("task %s not found", name)
	return false
}

func setCronTaskStatusForTest(statuses map[string]uint8) func() {
	previous := loadCronTaskStatus
	loadCronTaskStatus = func(_ context.Context, taskCode string) (uint8, bool, error) {
		status, ok := statuses[taskCode]
		return status, ok, nil
	}
	return func() {
		loadCronTaskStatus = previous
	}
}
