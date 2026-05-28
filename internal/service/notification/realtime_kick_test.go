package notification

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

// newServerClientPair 建立一对 server/client websocket 连接，返回 server 端连接用于注册到 hub，
// client 端用于断言收到的 Close 帧。
func newServerClientPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()
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

// expectCloseCode 断言 client 端读到带指定 code 的 Close 帧。
func expectCloseCode(t *testing.T, conn *websocket.Conn, code int) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err := conn.ReadMessage()
	if !websocket.IsCloseError(err, code) {
		t.Fatalf("expected close code %d, got %v", code, err)
	}
}

// expectStillOpen 断言 client 端在短时间内未收到 Close 帧（连接保持）。
func expectStillOpen(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	if websocket.IsCloseError(err, auth.SessionControlCodeKicked, auth.SessionControlCodeExpired) {
		t.Fatalf("expected connection to stay open, but got close: %v", err)
	}
}

func TestHubKickSingleSessionOnlyKicksTarget(t *testing.T) {
	targetServer, targetClient, cleanupTarget := newServerClientPair(t)
	defer cleanupTarget()
	otherServer, otherClient, cleanupOther := newServerClientPair(t)
	defer cleanupOther()

	h := &hub{clients: make(map[uint]map[*wsClient]struct{})}
	h.register(&wsClient{userID: 7, sessionID: "sess-target", conn: targetServer})
	h.register(&wsClient{userID: 7, sessionID: "sess-other", conn: otherServer})

	h.kick(auth.SessionControl{UserID: 7, SessionID: "sess-target", Code: auth.SessionControlCodeKicked})

	expectCloseCode(t, targetClient, auth.SessionControlCodeKicked)
	expectStillOpen(t, otherClient)

	h.mu.RLock()
	remaining := len(h.clients[7])
	h.mu.RUnlock()
	if remaining != 1 {
		t.Fatalf("expected 1 remaining session, got %d", remaining)
	}
}

func TestHubKickAllSessionsWhenSessionIDEmpty(t *testing.T) {
	server1, client1, cleanup1 := newServerClientPair(t)
	defer cleanup1()
	server2, client2, cleanup2 := newServerClientPair(t)
	defer cleanup2()

	h := &hub{clients: make(map[uint]map[*wsClient]struct{})}
	h.register(&wsClient{userID: 9, sessionID: "sess-a", conn: server1})
	h.register(&wsClient{userID: 9, sessionID: "sess-b", conn: server2})

	h.kick(auth.SessionControl{UserID: 9, SessionID: "", Code: auth.SessionControlCodeKicked})

	expectCloseCode(t, client1, auth.SessionControlCodeKicked)
	expectCloseCode(t, client2, auth.SessionControlCodeKicked)

	h.mu.RLock()
	_, exists := h.clients[9]
	h.mu.RUnlock()
	if exists {
		t.Fatal("expected all sessions of user removed after all-endpoint kick")
	}
}

// 多标签页共用同一 token（同一 jti）时，两条连接都应被注册且都能被踢，
// 验证内层集合结构不会因 sessionID 相同而互相覆盖。
func TestHubKickHandlesDuplicateSessionID(t *testing.T) {
	server1, client1, cleanup1 := newServerClientPair(t)
	defer cleanup1()
	server2, client2, cleanup2 := newServerClientPair(t)
	defer cleanup2()

	h := &hub{clients: make(map[uint]map[*wsClient]struct{})}
	h.register(&wsClient{userID: 5, sessionID: "same-jti", conn: server1})
	h.register(&wsClient{userID: 5, sessionID: "same-jti", conn: server2})

	h.mu.RLock()
	registered := len(h.clients[5])
	h.mu.RUnlock()
	if registered != 2 {
		t.Fatalf("expected 2 registered connections for duplicate jti, got %d", registered)
	}

	h.kick(auth.SessionControl{UserID: 5, SessionID: "same-jti", Code: auth.SessionControlCodeExpired})

	expectCloseCode(t, client1, auth.SessionControlCodeExpired)
	expectCloseCode(t, client2, auth.SessionControlCodeExpired)
}
