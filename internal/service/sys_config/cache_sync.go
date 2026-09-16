package sys_config

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/redis/go-redis/v9"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/redisutil"
	"go.uber.org/zap"
)

const (
	sysConfigCacheSyncChannel    = "sys_config:cache:refresh"
	sysConfigCacheSyncTimeout    = 3 * time.Second
	sysConfigSubscriptionRefresh = time.Second
)

type sysConfigCacheSyncPayload struct {
	Source    string `json:"source"`
	Timestamp int64  `json:"timestamp"`
}

var (
	sysConfigCacheSyncSourceID = uuid.New().String()
	sysConfigReconcileInterval = 30 * time.Second
	reloadSysConfigCache       func() error
	sysConfigSubscriberState   = struct {
		mu      sync.Mutex
		started bool
		cancel  context.CancelFunc
		done    chan struct{}
	}{}
)

func init() {
	reloadSysConfigCache = func() error { return NewSysConfigService().refreshCache(false) }
}

func notifySysConfigCacheRefreshed() {
	ensureSysConfigCacheSyncSubscriber()

	cfg := config.GetConfig()
	if cfg == nil || !cfg.Redis.Enable {
		return
	}
	client := data.RedisClient()
	if client == nil {
		return
	}

	payload, err := json.Marshal(sysConfigCacheSyncPayload{
		Source:    sysConfigCacheSyncSourceID,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), sysConfigCacheSyncTimeout)
	defer cancel()
	if err := client.Publish(ctx, redisutil.Channel(sysConfigCacheSyncChannel), payload).Err(); err != nil && log.Logger != nil {
		log.Logger.Warn("发布系统参数缓存刷新通知失败", zap.Error(err))
	}
}

func ensureSysConfigCacheSyncSubscriber() {
	sysConfigSubscriberState.mu.Lock()
	if sysConfigSubscriberState.started {
		sysConfigSubscriberState.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	sysConfigSubscriberState.started = true
	sysConfigSubscriberState.cancel = cancel
	sysConfigSubscriberState.done = make(chan struct{})
	sysConfigSubscriberState.mu.Unlock()

	go runSysConfigCacheSyncSubscriber(ctx)
}

func StopSysConfigCacheSyncSubscriber(timeout time.Duration) {
	sysConfigSubscriberState.mu.Lock()
	cancel := sysConfigSubscriberState.cancel
	done := sysConfigSubscriberState.done
	sysConfigSubscriberState.mu.Unlock()

	if cancel == nil || done == nil {
		return
	}
	cancel()
	if timeout <= 0 {
		<-done
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		if log.Logger != nil {
			log.Logger.Warn("系统参数缓存刷新订阅退出超时", zap.Duration("timeout", timeout))
		}
	}
}

func runSysConfigCacheSyncSubscriber(ctx context.Context) {
	runSysConfigCacheSyncSubscriberWithSource(ctx, data.RedisClient, sysConfigSubscriptionRefresh, sysConfigReconcileInterval)
}

func runSysConfigCacheSyncSubscriberWithSource(ctx context.Context, clientProvider func() *redis.Client, subscriptionRefreshInterval, reconcileInterval time.Duration) {
	defer func() {
		sysConfigSubscriberState.mu.Lock()
		done := sysConfigSubscriberState.done
		sysConfigSubscriberState.started = false
		sysConfigSubscriberState.cancel = nil
		sysConfigSubscriberState.done = nil
		sysConfigSubscriberState.mu.Unlock()
		if done != nil {
			close(done)
		}
	}()

	reconcileSysConfigCache(ctx, clientProvider, subscriptionRefreshInterval, reconcileInterval)
}

func reconcileSysConfigCache(ctx context.Context, clientProvider func() *redis.Client, subscriptionRefreshInterval, reconcileInterval time.Duration) {
	// Pub/Sub 是至多一次投递；首次订阅和定时回源弥补断线或刷新失败期间丢失的通知。
	refresh := func() {
		if err := reloadSysConfigCache(); err != nil && log.Logger != nil {
			log.Logger.Warn("系统参数缓存回源刷新失败，将定时重试", zap.Error(err))
		}
	}
	refresh()
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	subscriptionTicker := time.NewTicker(subscriptionRefreshInterval)
	defer subscriptionTicker.Stop()
	var subscription redisutil.DynamicSubscription
	defer subscription.Close()
	messages := subscription.Refresh(ctx, clientProvider(), redisutil.Channel(sysConfigCacheSyncChannel))
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		case <-subscriptionTicker.C:
			messages = subscription.Refresh(ctx, clientProvider(), redisutil.Channel(sysConfigCacheSyncChannel))
		case message, open := <-messages:
			if !open {
				subscription.Reset()
				messages = nil
				continue
			}
			payload, ok := decodeSysConfigCacheSyncPayload(message.Payload)
			if ok && payload.Source != sysConfigCacheSyncSourceID {
				refresh()
			}
		}
	}
}

func decodeSysConfigCacheSyncPayload(raw string) (sysConfigCacheSyncPayload, bool) {
	payload := sysConfigCacheSyncPayload{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return payload, true
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, false
	}
	return payload, true
}
