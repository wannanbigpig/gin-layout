package sys_config

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestDecodeSysConfigCacheSyncPayload(t *testing.T) {
	payload, ok := decodeSysConfigCacheSyncPayload(`{"source":"node-1","timestamp":123}`)
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if payload.Source != "node-1" || payload.Timestamp != 123 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestDecodeSysConfigCacheSyncPayloadRejectsInvalidJSON(t *testing.T) {
	_, ok := decodeSysConfigCacheSyncPayload("{invalid")
	if ok {
		t.Fatal("expected invalid payload to be rejected")
	}
}

func TestStopSysConfigCacheSyncSubscriberClosesBlockingReceive(t *testing.T) {
	server := newBlockingRedisPubSubServer(t)
	client := redis.NewClient(&redis.Options{
		Addr:            server.addr,
		Protocol:        2,
		DisableIdentity: true,
		ReadTimeout:     -1,
		WriteTimeout:    time.Second,
	})
	defer client.Close()

	core, observed := observer.New(zap.WarnLevel)
	restoreLogger := log.ReplaceLoggerForTesting(zap.New(core))
	defer restoreLogger()
	resetSysConfigSubscriberStateForTesting(t)

	ctx, cancel := context.WithCancel(context.Background())
	sysConfigSubscriberState.mu.Lock()
	sysConfigSubscriberState.started = true
	sysConfigSubscriberState.cancel = cancel
	sysConfigSubscriberState.done = make(chan struct{})
	done := sysConfigSubscriberState.done
	sysConfigSubscriberState.mu.Unlock()

	go runSysConfigCacheSyncSubscriberWithSource(ctx, func() *redis.Client { return client }, 10*time.Millisecond, time.Hour)
	server.waitForSubscriptions(t)

	start := time.Now()
	StopSysConfigCacheSyncSubscriber(500 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("expected subscriber stop to return quickly, took %s", elapsed)
	}

	select {
	case <-done:
	default:
		t.Fatal("expected subscriber done channel to be closed")
	}
	for _, entry := range observed.All() {
		if strings.Contains(entry.Message, "系统参数缓存刷新订阅退出超时") ||
			strings.Contains(entry.Message, "系统参数缓存刷新订阅中断") {
			t.Fatalf("expected shutdown path not to warn, got %q", entry.Message)
		}
	}
}

func TestSysConfigSubscriberReconcilesWithoutNotifications(t *testing.T) {
	originalReload := reloadSysConfigCache
	defer func() { reloadSysConfigCache = originalReload }()
	var calls atomic.Int32
	reloadSysConfigCache = func() error { calls.Add(1); return fmt.Errorf("temporary refresh failure") }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		reconcileSysConfigCache(ctx, func() *redis.Client { return nil }, 10*time.Millisecond, 10*time.Millisecond)
	}()
	defer func() { cancel(); <-done }()
	deadline := time.After(time.Second)
	for calls.Load() < 2 {
		select {
		case <-deadline:
			t.Fatal("failed initial refresh was not retried without notification")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestSysConfigSubscriberSwitchesToCurrentRedisClient(t *testing.T) {
	firstServer := newBlockingRedisPubSubServer(t)
	secondServer := newBlockingRedisPubSubServer(t)
	firstClient := redis.NewClient(&redis.Options{Addr: firstServer.addr, Protocol: 2, DisableIdentity: true, ReadTimeout: -1})
	secondClient := redis.NewClient(&redis.Options{Addr: secondServer.addr, Protocol: 2, DisableIdentity: true, ReadTimeout: -1})
	defer firstClient.Close()
	defer secondClient.Close()

	originalReload := reloadSysConfigCache
	defer func() { reloadSysConfigCache = originalReload }()
	reloadSysConfigCache = func() error { return nil }

	var current atomic.Pointer[redis.Client]
	current.Store(firstClient)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		reconcileSysConfigCache(ctx, current.Load, 10*time.Millisecond, time.Hour)
	}()
	defer func() { cancel(); <-done }()

	firstServer.waitForSubscriptions(t)
	current.Store(secondClient)
	secondServer.waitForSubscriptions(t)
}

func resetSysConfigSubscriberStateForTesting(t *testing.T) {
	t.Helper()
	StopSysConfigCacheSyncSubscriber(time.Second)
	t.Cleanup(func() { StopSysConfigCacheSyncSubscriber(time.Second) })
}

type blockingRedisPubSubServer struct {
	addr       string
	subscribed chan struct{}
}

func newBlockingRedisPubSubServer(t *testing.T) *blockingRedisPubSubServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake redis failed: %v", err)
	}
	server := &blockingRedisPubSubServer{
		addr:       listener.Addr().String(),
		subscribed: make(chan struct{}),
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		for {
			args, err := readRedisArray(reader)
			if err != nil {
				return
			}
			if len(args) == 0 {
				continue
			}
			switch strings.ToLower(args[0]) {
			case "hello":
				_, _ = conn.Write([]byte("-ERR unknown command 'HELLO'\r\n"))
			case "subscribe":
				for i, channel := range args[1:] {
					_, _ = fmt.Fprintf(conn, "*3\r\n$9\r\nsubscribe\r\n$%d\r\n%s\r\n:%d\r\n", len(channel), channel, i+1)
				}
				close(server.subscribed)
			default:
				_, _ = conn.Write([]byte("+OK\r\n"))
			}
		}
	}()

	return server
}

func (s *blockingRedisPubSubServer) waitForSubscriptions(t *testing.T) {
	t.Helper()
	select {
	case <-s.subscribed:
	case <-time.After(time.Second):
		t.Fatal("wait fake redis subscription timeout")
	}
}

func readRedisArray(reader *bufio.Reader) ([]string, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(header), "*%d", &count); err != nil {
		return nil, err
	}
	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		bulkHeader, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		var size int
		if _, err := fmt.Sscanf(strings.TrimSpace(bulkHeader), "$%d", &size); err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:size]))
	}
	return args, nil
}
