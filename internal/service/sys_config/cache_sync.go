package sys_config

import (
	"context"
	"encoding/json"
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
	sysConfigCacheSyncChannel = "sys_config:cache:refresh"
	sysConfigCacheSyncTimeout = 3 * time.Second
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
	sysConfigSubscriberState.started = true
	sysConfigSubscriberState.mu.Unlock()

	go runSysConfigCacheSyncSubscriber(client)
}

func runSysConfigCacheSyncSubscriber(client *redis.Client) {
	ctx := context.Background()
	pubsub := client.Subscribe(ctx, sysConfigCacheSyncChannel)
	defer func() {
		_ = pubsub.Close()
		sysConfigSubscriberState.mu.Lock()
		sysConfigSubscriberState.started = false
		sysConfigSubscriberState.mu.Unlock()
	}()

	if _, err := pubsub.Receive(ctx); err != nil {
		if log.Logger != nil {
			log.Logger.Warn("订阅系统参数缓存刷新通道失败", zap.Error(err))
		}
		return
	}

	for {
		message, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if log.Logger != nil {
				log.Logger.Warn("系统参数缓存刷新订阅中断", zap.Error(err))
			}
			return
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
