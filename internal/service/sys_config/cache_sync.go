package sys_config

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	sysConfigCacheSyncChannel    = "sys_config:cache:refresh"
	sysConfigCacheSyncTimeout    = 3 * time.Second
	sysConfigSubscribeMaxBackoff = 30 * time.Second
)

type sysConfigCacheSyncPayload struct {
	Source    string `json:"source"`
	Timestamp int64  `json:"timestamp"`
}

var (
	sysConfigCacheSyncSourceID = buildSysConfigCacheSyncSourceID()
	sysConfigSubscriberState   = struct {
		mu      sync.Mutex
		started bool
		cancel  context.CancelFunc
		done    chan struct{}
	}{}
)

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
	if err := client.Publish(ctx, sysConfigCacheSyncChannel, payload).Err(); err != nil && log.Logger != nil {
		log.Logger.Warn("发布系统参数缓存刷新通知失败", zap.Error(err))
	}
}

func ensureSysConfigCacheSyncSubscriber() {
	cfg := config.GetConfig()
	if cfg == nil || !cfg.Redis.Enable {
		return
	}
	client := data.RedisClient()
	if client == nil {
		return
	}

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

	go runSysConfigCacheSyncSubscriber(ctx, client)
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

func runSysConfigCacheSyncSubscriber(ctx context.Context, client *redis.Client) {
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

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		connected, err := receiveSysConfigCacheChanges(ctx, client)
		if isSysConfigCacheSyncClosing(ctx, err) {
			return
		}
		if connected {
			backoff = time.Second
		}
		if log.Logger != nil {
			log.Logger.Warn("系统参数缓存刷新订阅中断，准备重连",
				zap.Duration("backoff", backoff),
				zap.Error(err))
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < sysConfigSubscribeMaxBackoff {
			backoff *= 2
			if backoff > sysConfigSubscribeMaxBackoff {
				backoff = sysConfigSubscribeMaxBackoff
			}
		}
	}
}

func receiveSysConfigCacheChanges(ctx context.Context, client *redis.Client) (connected bool, err error) {
	pubsub := client.Subscribe(ctx, sysConfigCacheSyncChannel)
	defer func() { _ = pubsub.Close() }()
	stopClosing := closePubSubOnContextDone(ctx, pubsub)
	defer stopClosing()

	if _, err := pubsub.Receive(ctx); err != nil {
		if isSysConfigCacheSyncClosing(ctx, err) {
			return false, err
		}
		if log.Logger != nil {
			log.Logger.Warn("订阅系统参数缓存刷新通道失败", zap.Error(err))
		}
		return false, err
	}

	for {
		message, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if isSysConfigCacheSyncClosing(ctx, err) {
				return true, err
			}
			if log.Logger != nil {
				log.Logger.Warn("系统参数缓存刷新订阅中断", zap.Error(err))
			}
			return true, err
		}

		payload, ok := decodeSysConfigCacheSyncPayload(message.Payload)
		if !ok || payload.Source == sysConfigCacheSyncSourceID {
			continue
		}
		if err := NewSysConfigService().refreshCache(false); err != nil && log.Logger != nil {
			log.Logger.Warn("处理系统参数缓存刷新通知失败", zap.Error(err))
		}
	}
}

func closePubSubOnContextDone(ctx context.Context, pubsub *redis.PubSub) func() {
	if ctx == nil || pubsub == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = pubsub.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func isSysConfigCacheSyncClosing(ctx context.Context, err error) bool {
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, net.ErrClosed) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "use of closed network connection") || strings.Contains(msg, "redis: client is closed")
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

func buildSysConfigCacheSyncSourceID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return host + ":" + strconv.Itoa(os.Getpid())
}
