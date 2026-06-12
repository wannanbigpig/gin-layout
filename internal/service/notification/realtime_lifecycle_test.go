package notification

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestHubUnregisterIsIdempotentUnderConcurrentCalls(t *testing.T) {
	serverConn, clientConn, cleanup := newRealtimeTestPair(t)
	defer cleanup()

	h := &hub{clients: make(map[uint]map[*wsClient]struct{}), maxConnsPerUser: wsMaxConnsPerUser, maxConnsTotal: wsMaxConnsTotal}
	client := &wsClient{userID: 7, conn: serverConn}
	_ = h.register(client)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.unregister(client)
		}()
	}
	wg.Wait()

	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) != 0 {
		t.Fatalf("expected hub clients to be cleared, got %#v", h.clients)
	}
	_ = clientConn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := clientConn.ReadMessage(); err == nil {
		t.Fatal("expected websocket peer to be closed after unregister")
	}
}

func TestShutdownRealtimeDispatcherSendsCloseFrame(t *testing.T) {
	serverConn, clientConn, cleanup := newRealtimeTestPair(t)
	defer cleanup()

	previousHub := defaultHub
	defaultHub = &hub{clients: make(map[uint]map[*wsClient]struct{}), maxConnsPerUser: wsMaxConnsPerUser, maxConnsTotal: wsMaxConnsTotal}
	defer func() { defaultHub = previousHub }()

	_ = defaultHub.register(&wsClient{userID: 11, conn: serverConn})

	ShutdownRealtimeDispatcher()

	_ = clientConn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := clientConn.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseGoingAway) {
		t.Fatalf("expected close going away error, got %v", err)
	}
}

func TestHubChannelSubscriberCountOnlyCountsMatchingSubscriptions(t *testing.T) {
	h := &hub{clients: make(map[uint]map[*wsClient]struct{}), maxConnsPerUser: wsMaxConnsPerUser, maxConnsTotal: wsMaxConnsTotal}
	monitorClient := &wsClient{userID: 1}
	otherClient := &wsClient{userID: 2}
	monitorClient.SubscribeChannel("monitor")
	otherClient.SubscribeChannel("other")
	_ = h.register(monitorClient)
	_ = h.register(otherClient)

	if got := h.channelSubscriberCount("monitor"); got != 1 {
		t.Fatalf("expected one monitor subscriber, got %d", got)
	}
	if got := h.channelSubscriberCount("missing"); got != 0 {
		t.Fatalf("expected zero missing-channel subscribers, got %d", got)
	}
}

func TestHandleClientMessageRepliesPongForApplicationPing(t *testing.T) {
	serverConn, clientConn, cleanup := newRealtimeTestPair(t)
	defer cleanup()

	client := &wsClient{userID: 7, conn: serverConn}
	handleClientMessage(client, []byte(`{"action":"ping"}`))

	_ = clientConn.SetReadDeadline(time.Now().Add(time.Second))
	_, raw, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("expected application pong message, got error: %v", err)
	}
	var msg GenericMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("decode pong message failed: %v", err)
	}
	if msg.Type != "pong" {
		t.Fatalf("expected pong message type, got %q", msg.Type)
	}
}

func TestHubRegisterEnforcesPerUserConnectionLimit(t *testing.T) {
	h := &hub{clients: make(map[uint]map[*wsClient]struct{}), maxConnsPerUser: 1, maxConnsTotal: 10}
	first := &wsClient{userID: 7}
	second := &wsClient{userID: 7}

	if err := h.register(first); err != nil {
		t.Fatalf("expected first connection to register, got %v", err)
	}
	if err := h.register(second); err == nil {
		t.Fatal("expected second same-user connection to be rejected")
	}
	if got := h.onlineCount(); got != 1 {
		t.Fatalf("expected one registered connection after reject, got %d", got)
	}
}

func TestHubRegisterEnforcesTotalConnectionLimit(t *testing.T) {
	h := &hub{clients: make(map[uint]map[*wsClient]struct{}), maxConnsPerUser: 10, maxConnsTotal: 1}
	first := &wsClient{userID: 7}
	second := &wsClient{userID: 8}

	if err := h.register(first); err != nil {
		t.Fatalf("expected first connection to register, got %v", err)
	}
	if err := h.register(second); err == nil {
		t.Fatal("expected second global connection to be rejected")
	}
	if got := h.onlineCount(); got != 1 {
		t.Fatalf("expected one registered connection after reject, got %d", got)
	}
}

func TestStopRealtimeSubscriberCancelsRunner(t *testing.T) {
	started := make(chan struct{}, 1)
	stopped := make(chan struct{}, 1)
	previousRunner := realtimeRunner
	realtimeRunner = func(ctx context.Context, _ *redis.Client) {
		started <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
	}
	defer func() {
		realtimeRunner = previousRunner
		StopRealtimeSubscriber(time.Second)
	}()

	if !startRealtimeSubscriber(&redis.Client{}) {
		t.Fatal("expected realtime subscriber to start")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected realtime runner to start")
	}

	StopRealtimeSubscriber(time.Second)

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("expected realtime runner to stop after cancel")
	}
}

func TestRunRealtimeSubscriberStopsBlockingReceiveWithoutWarn(t *testing.T) {
	server := newBlockingRealtimeRedisPubSubServer(t)
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

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runRealtimeSubscriber(ctx, client)
	}()
	server.waitForSubscriptions(t)

	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected realtime subscriber to stop quickly after cancel")
	}
	for _, entry := range observed.All() {
		if strings.Contains(entry.Message, "系统消息实时订阅中断") ||
			strings.Contains(entry.Message, "订阅系统消息实时通道失败") {
			t.Fatalf("expected shutdown path not to warn, got %q", entry.Message)
		}
	}
}

func TestRealtimeSubscriberClosingErrorIsSilent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if !isRealtimeSubscriberClosing(ctx, fmt.Errorf("read tcp 127.0.0.1:1->127.0.0.1:2: use of closed network connection")) {
		t.Fatal("expected canceled context with closed connection error to be treated as closing")
	}
}

func newRealtimeTestPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()

	// 测试环境中替换 origin checker，避免依赖全局 config
	previousChecker := wsOriginChecker
	wsOriginChecker = func(*http.Request) bool { return true }

	serverConnCh := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		serverConnCh <- conn
	}))

	wsURL := "ws" + server.URL[len("http"):]
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket failed: %v", err)
	}

	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case <-time.After(time.Second):
		_ = clientConn.Close()
		server.Close()
		t.Fatal("wait server websocket conn timeout")
	}

	cleanup := func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
		server.Close()
		wsOriginChecker = previousChecker
	}
	return serverConn, clientConn, cleanup
}

type blockingRealtimeRedisPubSubServer struct {
	addr       string
	subscribed chan struct{}
}

func newBlockingRealtimeRedisPubSubServer(t *testing.T) *blockingRealtimeRedisPubSubServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake redis failed: %v", err)
	}
	server := &blockingRealtimeRedisPubSubServer{
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
			args, err := readRealtimeRedisArray(reader)
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

func (s *blockingRealtimeRedisPubSubServer) waitForSubscriptions(t *testing.T) {
	t.Helper()
	select {
	case <-s.subscribed:
	case <-time.After(time.Second):
		t.Fatal("wait fake redis subscription timeout")
	}
}

func readRealtimeRedisArray(reader *bufio.Reader) ([]string, error) {
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
