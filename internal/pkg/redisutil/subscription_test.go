package redisutil

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestDynamicSubscriptionRebuildsForClientAndChannelChanges(t *testing.T) {
	firstClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0", DialTimeout: time.Millisecond, MaxRetries: -1})
	secondClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0", DialTimeout: time.Millisecond, MaxRetries: -1})
	defer firstClient.Close()
	defer secondClient.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var subscription DynamicSubscription
	defer subscription.Close()

	firstMessages := subscription.Refresh(ctx, firstClient, "app:test:db0:events")
	firstPubSub := subscription.pubsub
	if firstMessages == nil || firstPubSub == nil {
		t.Fatal("expected initial subscription")
	}
	if got := subscription.Refresh(ctx, firstClient, "app:test:db0:events"); got != firstMessages || subscription.pubsub != firstPubSub {
		t.Fatal("unchanged source must reuse subscription")
	}

	if got := subscription.Refresh(ctx, firstClient, "app:prod:db0:events"); got == nil || subscription.pubsub == firstPubSub {
		t.Fatal("channel namespace change must rebuild subscription")
	}
	secondPubSub := subscription.pubsub
	if got := subscription.Refresh(ctx, secondClient, "app:prod:db0:events"); got == nil || subscription.pubsub == secondPubSub {
		t.Fatal("redis client change must rebuild subscription")
	}

	subscription.Refresh(ctx, nil, "app:prod:db0:events")
	if subscription.pubsub != nil || subscription.messages != nil {
		t.Fatal("missing redis client must close current subscription")
	}
}
