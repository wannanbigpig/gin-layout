package casbinx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	"github.com/redis/go-redis/v9"
)

func TestPolicySyncReconcilesAfterNotificationAndMissedEvent(t *testing.T) {
	messages := make(chan *redis.Message, 1)
	loaded := make(chan struct{}, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		reconcilePolicy(ctx, messages, 20*time.Millisecond, func() error { loaded <- struct{}{}; return errors.New("temporary failure") })
	}()
	defer func() { cancel(); <-done }()
	wait := func() {
		t.Helper()
		select {
		case <-loaded:
		case <-time.After(time.Second):
			t.Fatal("policy was not reconciled")
		}
	}
	wait() // 启动加载失败仍需恢复。
	messages <- &redis.Message{Payload: "another-api-instance"}
	wait()
	wait() // 无后续通知也会回源，覆盖丢失通知和数据库短暂故障。
}

type sharedTestPolicyAdapter struct{ allowed atomic.Bool }

func (a *sharedTestPolicyAdapter) LoadPolicy(m casbinmodel.Model) error {
	if a.allowed.Load() {
		return persist.LoadPolicyLine("p, user, /resource, GET", m)
	}
	return nil
}
func (*sharedTestPolicyAdapter) SavePolicy(casbinmodel.Model) error          { return nil }
func (*sharedTestPolicyAdapter) AddPolicy(string, string, []string) error    { return nil }
func (*sharedTestPolicyAdapter) RemovePolicy(string, string, []string) error { return nil }
func (*sharedTestPolicyAdapter) RemoveFilteredPolicy(string, string, int, ...string) error {
	return nil
}

func TestIndependentPolicyReplicasConvergeAfterRevocation(t *testing.T) {
	adapter := &sharedTestPolicyAdapter{}
	adapter.allowed.Store(true)
	newReplica := func() *CasbinEnforcer {
		m, err := casbinmodel.NewModelFromString("[request_definition]\nr = sub, obj, act\n[policy_definition]\np = sub, obj, act\n[policy_effect]\ne = some(where (p.eft == allow))\n[matchers]\nm = r.sub == p.sub && r.obj == p.obj && r.act == p.act")
		if err != nil {
			t.Fatal(err)
		}
		e, err := casbin.NewEnforcer(m, adapter)
		if err != nil {
			t.Fatal(err)
		}
		return &CasbinEnforcer{Enforcer: e, policyState: &sharedPolicyState{}}
	}
	first, second := newReplica(), newReplica()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	messages := make(chan *redis.Message, 1)
	ready := make(chan struct{})
	var readyOnce sync.Once
	go func() {
		defer close(done)
		reconcilePolicy(ctx, messages, 20*time.Millisecond, func() error {
			err := second.reloadPolicy()
			readyOnce.Do(func() { close(ready) })
			return err
		})
	}()
	defer func() { cancel(); <-done }()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("replica did not start")
	}
	adapter.allowed.Store(false)
	if err := first.reloadPolicy(); err != nil {
		t.Fatal(err)
	}
	// 刻意丢失通知，另一副本也应通过定时回源取消旧授权。
	deadline := time.After(time.Second)
	for {
		allowed, err := second.Enforce("user", "/resource", "GET")
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("replica kept revoked permission")
		case <-time.After(time.Millisecond):
		}
	}
}
