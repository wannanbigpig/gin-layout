package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestRuntimeCronRegistryReconcilesDefinitionChanges(t *testing.T) {
	crontab := cron.New(cron.WithSeconds())
	registry := newRuntimeCronRegistry(context.Background(), crontab, &cronLogger{})

	restoreDefinitions := setCronTaskDefinitionsForTest(t, []cronTaskDefinition{
		{
			TaskCode: "cron:demo",
			TaskName: "demo",
			CronSpec: "0 0 4 * * *",
			Handler:  "cron.demo",
			Status:   model.TaskStatusEnabled,
		},
	})
	registry.reconcileOnce(context.Background())

	if len(registry.tasks) != 1 || len(crontab.Entries()) != 1 {
		t.Fatalf("expected one runtime task, got tasks=%d entries=%d", len(registry.tasks), len(crontab.Entries()))
	}
	firstEntry := registry.tasks["cron:demo"].entryID

	restoreDefinitions()
	restoreDefinitions = setCronTaskDefinitionsForTest(t, []cronTaskDefinition{
		{
			TaskCode: "cron:demo",
			TaskName: "demo",
			CronSpec: "0 0 5 * * *",
			Handler:  "cron.demo",
			Status:   model.TaskStatusEnabled,
		},
	})
	registry.reconcileOnce(context.Background())

	if len(registry.tasks) != 1 || registry.tasks["cron:demo"].definition.CronSpec != "0 0 5 * * *" {
		t.Fatalf("expected runtime task spec to be replaced, got %#v", registry.tasks["cron:demo"])
	}
	if registry.tasks["cron:demo"].entryID == firstEntry {
		t.Fatal("expected cron entry to be replaced when cron spec changes")
	}

	restoreDefinitions()
	restoreDefinitions = setCronTaskDefinitionsForTest(t, []cronTaskDefinition{
		{
			TaskCode: "cron:demo",
			TaskName: "demo",
			CronSpec: "0 0 5 * * *",
			Handler:  "cron.demo",
			Status:   model.TaskStatusDisabled,
		},
	})
	registry.reconcileOnce(context.Background())

	if len(registry.tasks) != 0 || len(crontab.Entries()) != 0 {
		t.Fatalf("expected disabled task to be removed, got tasks=%d entries=%d", len(registry.tasks), len(crontab.Entries()))
	}
	restoreDefinitions()
}

func TestRuntimeCronRegistryKeepsLowRiskTaskRunningWhenDefinitionLoadFails(t *testing.T) {
	registry := newRuntimeRegistryWithTask(t, cronTaskDefinition{
		TaskCode: "cron:demo",
		TaskName: "demo",
		CronSpec: "0 0 4 * * *",
		Handler:  "cron.demo",
		Status:   model.TaskStatusEnabled,
	})
	restoreDefinitions := setCronTaskDefinitionsErrorForTest(t, errors.New("db down"))
	registry.reconcileOnce(context.Background())
	restoreDefinitions()

	if len(registry.tasks) != 1 {
		t.Fatalf("expected existing task to be kept, got %d", len(registry.tasks))
	}
	if !registry.taskCanRun("cron:demo", "0 0 4 * * *") {
		t.Fatal("expected low-risk task to keep running from cache")
	}
}

func TestRuntimeCronRegistryFailClosesHighRiskTaskWhenDefinitionLoadFails(t *testing.T) {
	registry := newRuntimeRegistryWithTask(t, cronTaskDefinition{
		TaskCode:   "cron:reset-system-data",
		TaskName:   "reset-system-data",
		CronSpec:   "0 0 4 * * *",
		Handler:    "cron.reset-system-data",
		Status:     model.TaskStatusEnabled,
		IsHighRisk: model.TaskHighRisk,
	})
	restoreDefinitions := setCronTaskDefinitionsErrorForTest(t, errors.New("db down"))
	registry.reconcileOnce(context.Background())
	restoreDefinitions()

	if registry.taskCanRun("cron:reset-system-data", "0 0 4 * * *") {
		t.Fatal("expected high-risk task to fail closed when sync failed")
	}
}

func TestRuntimeCronRegistryFailClosesHighRiskTaskWhenCacheIsStale(t *testing.T) {
	registry := newRuntimeRegistryWithTask(t, cronTaskDefinition{
		TaskCode:   "cron:reset-system-data",
		TaskName:   "reset-system-data",
		CronSpec:   "0 0 4 * * *",
		Handler:    "cron.reset-system-data",
		Status:     model.TaskStatusEnabled,
		IsHighRisk: model.TaskHighRisk,
	})
	registry.mu.Lock()
	current := registry.tasks["cron:reset-system-data"]
	current.loadedAt = time.Now().Add(-cronRuntimeCacheMaxAge - time.Second)
	registry.tasks["cron:reset-system-data"] = current
	registry.mu.Unlock()

	if registry.taskCanRun("cron:reset-system-data", "0 0 4 * * *") {
		t.Fatal("expected high-risk task to fail closed when cache is stale")
	}
}

func TestRuntimeCronRegistryRejectsInvalidSpec(t *testing.T) {
	crontab := cron.New(cron.WithSeconds())
	registry := newRuntimeCronRegistry(context.Background(), crontab, &cronLogger{})
	restoreDefinitions := setCronTaskDefinitionsForTest(t, []cronTaskDefinition{
		{
			TaskCode: "cron:demo",
			TaskName: "demo",
			CronSpec: "bad spec",
			Handler:  "cron.demo",
			Status:   model.TaskStatusEnabled,
		},
	})
	registry.reconcileOnce(context.Background())
	restoreDefinitions()

	if len(registry.tasks) != 0 || len(crontab.Entries()) != 0 {
		t.Fatalf("expected invalid task not to be registered, got tasks=%d entries=%d", len(registry.tasks), len(crontab.Entries()))
	}
}

func TestRuntimeCronRegistryRuntimeStateUpdatesOnlyWhenChanged(t *testing.T) {
	registry := newRuntimeCronRegistry(context.Background(), cron.New(cron.WithSeconds()), &cronLogger{})
	definition := cronTaskDefinition{
		TaskCode: "cron:demo",
		TaskName: "demo",
		CronSpec: "0 0 4 * * *",
		Handler:  "cron.demo",
		Status:   model.TaskStatusEnabled,
	}

	updates := registry.appendRuntimeStateUpdateIfChanged(nil, definition, model.TaskStatusEnabled, cronRuntimeSyncSuccess, nil)
	if len(updates) != 1 {
		t.Fatalf("expected first state update, got %d", len(updates))
	}
	registry.markRuntimeStatesRecorded(updates)

	updates = registry.appendRuntimeStateUpdateIfChanged(nil, definition, model.TaskStatusEnabled, cronRuntimeSyncSuccess, nil)
	if len(updates) != 0 {
		t.Fatalf("expected unchanged state to skip update, got %d", len(updates))
	}

	updates = registry.appendRuntimeStateUpdateIfChanged(nil, definition, model.TaskStatusDisabled, cronRuntimeSyncSuccess, nil)
	if len(updates) != 1 {
		t.Fatalf("expected active status change to record update, got %d", len(updates))
	}
}

func newRuntimeRegistryWithTask(t *testing.T, definition cronTaskDefinition) *runtimeCronRegistry {
	t.Helper()
	crontab := cron.New(cron.WithSeconds())
	registry := newRuntimeCronRegistry(context.Background(), crontab, &cronLogger{})
	restoreDefinitions := setCronTaskDefinitionsForTest(t, []cronTaskDefinition{definition})
	registry.reconcileOnce(context.Background())
	restoreDefinitions()
	if len(registry.tasks) != 1 {
		t.Fatalf("expected one runtime task, got %d", len(registry.tasks))
	}
	return registry
}

func setCronTaskDefinitionsForTest(t *testing.T, definitions []cronTaskDefinition) func() {
	t.Helper()
	previous := loadCronTaskDefinitions
	loadCronTaskDefinitions = func(context.Context) ([]cronTaskDefinition, error) {
		return definitions, nil
	}
	return func() {
		loadCronTaskDefinitions = previous
	}
}

func setCronTaskDefinitionsErrorForTest(t *testing.T, err error) func() {
	t.Helper()
	previous := loadCronTaskDefinitions
	loadCronTaskDefinitions = func(context.Context) ([]cronTaskDefinition, error) {
		return nil, err
	}
	return func() {
		loadCronTaskDefinitions = previous
	}
}
