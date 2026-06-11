package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
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
