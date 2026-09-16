package casbinx

import (
	"context"
	"sync"
	"time"
	"uuid"

	"github.com/redis/go-redis/v9"
	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/redisutil"
	"go.uber.org/zap"
)

const policyReconcileInterval = 30 * time.Second

const policySubscriptionRefreshInterval = time.Second

var policySyncSource = uuid.New().String()
var policySyncState struct {
	sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// StartPolicySync 启动单个 API 进程的策略订阅；定时读数据库弥补通知丢失。
func StartPolicySync() {
	policySyncState.Lock()
	defer policySyncState.Unlock()
	if policySyncState.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	policySyncState.cancel, policySyncState.done = cancel, done
	go func() {
		defer close(done)
		messages := watchPolicyChanges(ctx, data.RedisClient, policySubscriptionRefreshInterval)
		reconcilePolicy(ctx, messages, policyReconcileInterval, func() error {
			enforcer, err := GetEnforcer()
			if err != nil {
				return err
			}
			return enforcer.reloadPolicy()
		})
	}()
}

func watchPolicyChanges(ctx context.Context, clientProvider func() *redis.Client, refreshInterval time.Duration) <-chan *redis.Message {
	events := make(chan *redis.Message, 1)
	go func() {
		defer close(events)
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		var subscription redisutil.DynamicSubscription
		defer subscription.Close()
		messages := subscription.Refresh(ctx, clientProvider(), redisutil.Channel("casbin:policy:changed"))
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				messages = subscription.Refresh(ctx, clientProvider(), redisutil.Channel("casbin:policy:changed"))
			case message, open := <-messages:
				if !open {
					subscription.Reset()
					messages = nil
					continue
				}
				if message == nil || message.Payload == policySyncSource {
					continue
				}
				// 策略通知只表达“需要重新加载”，积压时合并重复事件即可。
				select {
				case events <- message:
				default:
				}
			}
		}
	}()
	return events
}

func reconcilePolicy(ctx context.Context, messages <-chan *redis.Message, interval time.Duration, reload func() error) {
	reconcile := func() {
		if err := reload(); err != nil && log.Logger != nil {
			log.Logger.Warn("权限策略回源失败，暂时拒绝权限判定并等待重试", zap.Error(err))
		}
	}
	reconcile()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		case message, open := <-messages:
			if !open {
				messages = nil
				continue
			}
			if message != nil && message.Payload != policySyncSource {
				reconcile()
			}
		}
	}
}

func notifyPolicyChanged() {
	client := data.RedisClient()
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Publish(ctx, redisutil.Channel("casbin:policy:changed"), policySyncSource).Err(); err != nil && log.Logger != nil {
		log.Logger.Warn("权限策略通知失败，其他实例将定时回源", zap.Error(err))
	}
}

func StopPolicySync(timeout time.Duration) {
	policySyncState.Lock()
	cancel, done := policySyncState.cancel, policySyncState.done
	policySyncState.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		policySyncState.Lock()
		if policySyncState.done == done {
			policySyncState.cancel, policySyncState.done = nil, nil
		}
		policySyncState.Unlock()
	case <-timer.C:
		if log.Logger != nil {
			log.Logger.Warn("等待权限策略同步停止超时")
		}
	}
}
