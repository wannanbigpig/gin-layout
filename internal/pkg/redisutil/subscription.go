package redisutil

import (
	"context"
	"slices"

	"github.com/redis/go-redis/v9"
)

// DynamicSubscription 管理一个需要随 Redis client 或频道命名空间变化而重建的订阅。
// 实例应只由单个 goroutine 驱动；调用方负责定时调用 Refresh。
type DynamicSubscription struct {
	client   *redis.Client
	channels []string
	pubsub   *redis.PubSub
	messages <-chan *redis.Message
}

// Refresh 返回当前订阅消息通道。client 或频道变化时会先关闭旧订阅再重建。
func (s *DynamicSubscription) Refresh(ctx context.Context, client *redis.Client, channels ...string) <-chan *redis.Message {
	if client == nil || len(channels) == 0 {
		s.Reset()
		return nil
	}
	if s.client == client && slices.Equal(s.channels, channels) && s.messages != nil {
		return s.messages
	}

	s.Reset()
	s.client = client
	s.channels = append([]string(nil), channels...)
	s.pubsub = client.Subscribe(ctx, channels...)
	s.messages = s.pubsub.Channel()
	return s.messages
}

// Reset 丢弃当前订阅；下一次 Refresh 会重新读取 client 和频道并建立连接。
func (s *DynamicSubscription) Reset() {
	if s.pubsub != nil {
		_ = s.pubsub.Close()
	}
	s.client = nil
	s.channels = nil
	s.pubsub = nil
	s.messages = nil
}

// Close 关闭当前订阅。
func (s *DynamicSubscription) Close() {
	s.Reset()
}
