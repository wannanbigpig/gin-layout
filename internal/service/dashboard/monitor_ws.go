package dashboard

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/notification"
)

const (
	// monitorChannel 监控数据频道名称。
	monitorChannel = "monitor"
	// monitorPushInterval 监控数据推送间隔。
	monitorPushInterval = 2 * time.Second
)

var monitorPushState = struct {
	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
}{}

// EnsureMonitorPushStarted 确保监控数据推送已启动。
func EnsureMonitorPushStarted() {
	monitorPushState.mu.Lock()
	defer monitorPushState.mu.Unlock()

	if monitorPushState.started {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	monitorPushState.started = true
	monitorPushState.cancel = cancel
	monitorPushState.done = done

	go func() {
		defer close(done)
		defer func() {
			monitorPushState.mu.Lock()
			if monitorPushState.done == done {
				monitorPushState.started = false
				monitorPushState.cancel = nil
				monitorPushState.done = nil
			}
			monitorPushState.mu.Unlock()
		}()
		runMonitorPushLoop(ctx)
	}()
}

// StopMonitorPush 停止监控数据推送。
func StopMonitorPush(timeout time.Duration) {
	monitorPushState.mu.Lock()
	cancel := monitorPushState.cancel
	done := monitorPushState.done
	monitorPushState.mu.Unlock()

	if cancel == nil || done == nil {
		return
	}
	cancel()
	if timeout <= 0 {
		<-done
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

func runMonitorPushLoop(ctx context.Context) {
	ticker := time.NewTicker(monitorPushInterval)
	defer ticker.Stop()

	svc := NewMonitorService()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if notification.ChannelSubscriberCount(monitorChannel) == 0 {
				continue
			}
			snapshot, err := svc.GetSnapshot()
			if err != nil {
				if log.Logger != nil {
					log.Logger.Warn("获取监控快照失败", zap.Error(err))
				}
				continue
			}
			msg := notification.GenericMessage{
				Type: monitorChannel,
				Data: snapshot,
			}
			notification.BroadcastToChannel(monitorChannel, msg)
		}
	}
}
