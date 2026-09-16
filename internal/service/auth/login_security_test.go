package auth

import (
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

func TestShouldCountLockFailure(t *testing.T) {
	service := NewLoginService()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "wrong password", err: e.NewBusinessError(e.UserPasswordWrong), want: true},
		{name: "captcha", err: e.NewBusinessError(e.CaptchaErr), want: true},
		{name: "dependency not ready", err: e.NewBusinessError(e.ServiceDependencyNotReady), want: false},
		{name: "login failed", err: e.NewBusinessError(e.LoginFailed), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.shouldCountLockFailure(tc.err); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestApplyLoginFailStateLocksWhenThresholdReached(t *testing.T) {
	state := &model.LoginSecurityState{
		FailCount: 4,
	}
	now := time.Now()
	policy := loginLockPolicy{
		Enabled:      true,
		MaxFailures:  5,
		LockDuration: 15 * time.Minute,
	}

	applyLoginFailState(state, now, policy)

	if state.FailCount != 5 {
		t.Fatalf("expected fail count 5, got %d", state.FailCount)
	}
	if state.LockUntil == nil || !state.LockUntil.Time.After(now) {
		t.Fatal("expected lock_until to be set")
	}
	if state.LastFailedAt == nil || state.LastFailedAt.Time.IsZero() {
		t.Fatal("expected last_failed_at to be set")
	}
}

func TestApplyLoginFailStateResetsExpiredLock(t *testing.T) {
	now := time.Now()
	state := &model.LoginSecurityState{
		FailCount: 9,
		LockUntil: &utils.FormatDate{Time: now.Add(-time.Minute)},
	}
	policy := loginLockPolicy{
		Enabled:      true,
		MaxFailures:  5,
		LockDuration: 15 * time.Minute,
	}

	applyLoginFailState(state, now, policy)

	if state.FailCount != 1 {
		t.Fatalf("expected expired state to reset then increment to 1, got %d", state.FailCount)
	}
	if state.LockUntil != nil {
		t.Fatal("expected no lock while fail count below threshold")
	}
}

func TestLoginAllowedPreservesFailuresUntilThresholdAndClearsExpiredLock(t *testing.T) {
	db := newSessionTestDB(t)
	if err := db.Exec(`CREATE TABLE login_security_state (id integer primary key autoincrement, username text unique, fail_count integer default 0, lock_until datetime, last_failed_at datetime, created_at datetime, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	s := NewLoginServiceWithDeps(LoginServiceDeps{LoginLogDB: db})
	for i := 1; i <= defaultLoginMaxFailures; i++ {
		if err := s.CheckLoginAllowed("admin"); err != nil {
			t.Fatalf("attempt %d denied: %v", i, err)
		}
		if err := s.incrementLoginFailState("admin"); err != nil {
			t.Fatal(err)
		}
		var state model.LoginSecurityState
		if err := db.First(&state).Error; err != nil {
			t.Fatal(err)
		}
		if state.FailCount != uint(i) {
			t.Fatalf("fail count=%d want %d", state.FailCount, i)
		}
	}
	if err := s.CheckLoginAllowed("admin"); err == nil {
		t.Fatal("expected lock after consecutive failures")
	}
	if err := db.Table("login_security_state").Where("username = ?", "admin").Update("lock_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.CheckLoginAllowed("admin"); err != nil {
		t.Fatal(err)
	}
	var state model.LoginSecurityState
	if err := db.First(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state.FailCount != 0 || state.LockUntil != nil {
		t.Fatalf("expired lock not cleared: %#v", state)
	}
}
