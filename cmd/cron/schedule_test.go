package cron

import (
	"strings"
	"testing"

	"github.com/robfig/cron/v3"
)

func TestDailyAtSpecWithHHMM(t *testing.T) {
	spec, err := dailyAtSpec("02:30")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if spec != "0 30 2 * * *" {
		t.Fatalf("unexpected cron spec: %s", spec)
	}
}

func TestDailyAtSpecWithHHMMSS(t *testing.T) {
	spec, err := dailyAtSpec("03:04:05")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if spec != "5 4 3 * * *" {
		t.Fatalf("unexpected cron spec: %s", spec)
	}
}

func TestDailyAtSpecRejectsInvalidInput(t *testing.T) {
	_, err := dailyAtSpec("invalid")
	if err == nil {
		t.Fatal("expected invalid input to return error")
	}
}

func TestDailyAtSpecRejectsOutOfRange(t *testing.T) {
	_, err := dailyAtSpec("25:00")
	if err == nil {
		t.Fatal("expected out of range input to return error")
	}
}

func TestSchedulerRegisterReturnsDailyAtError(t *testing.T) {
	schedule := NewSchedule(&cronLogger{})
	schedule.Call("bad-task", func() {}).DailyAt("bad-time")

	c := cron.New(cron.WithSeconds())
	err := schedule.Register(c)
	if err == nil {
		t.Fatal("expected register to return error for invalid daily time")
	}
	if !strings.Contains(err.Error(), "调度表达式无效") {
		t.Fatalf("unexpected error: %v", err)
	}
}
