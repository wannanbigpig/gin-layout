package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	realtimeChannel      = "sys_messages:realtime"
	realtimePublishTTL   = 3 * time.Second
	realtimeWriteTimeout = 5 * time.Second
	realtimePingPeriod   = 30 * time.Second
)

type redisEventPayload struct {
	Source string `json:"source"`
	Event  Event  `json:"event"`
}

type wsClient struct {
	userID uint
	conn   *websocket.Conn
	mu     sync.Mutex
}

type hub struct {
	mu      sync.RWMutex
	clients map[uint]map[*wsClient]struct{}
}

var (
	defaultHub = &hub{clients: make(map[uint]map[*wsClient]struct{})}
	upgrader   = websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	realtimeSourceID = buildRealtimeSourceID()
	realtimeState    = struct {
		mu      sync.Mutex
		started bool
	}{}
)

func EnsureRealtimeDispatcherStarted() {
	cfg := config.GetConfig()
	if cfg == nil || !cfg.Redis.Enable {
		return
	}
	client := data.RedisClient()
	if client == nil {
		return
	}

	realtimeState.mu.Lock()
	if realtimeState.started {
		realtimeState.mu.Unlock()
		return
	}
	realtimeState.started = true
	realtimeState.mu.Unlock()

	go runRealtimeSubscriber(client)
}

func ServeWebsocket(w http.ResponseWriter, r *http.Request, userID uint) error {
	EnsureRealtimeDispatcherStarted()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	client := &wsClient{userID: userID, conn: conn}
	defaultHub.register(client)

	go client.keepalive()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			defaultHub.unregister(client)
			return nil
		}
	}
}

func BroadcastEvent(event Event) {
	defaultHub.broadcast(event)
}

func PublishEvent(ctx context.Context, event Event) error {
	cfg := config.GetConfig()
	if cfg == nil || !cfg.Redis.Enable {
		return nil
	}
	client := data.RedisClient()
	if client == nil {
		return nil
	}
	payload, err := json.Marshal(redisEventPayload{
		Source: realtimeSourceID,
		Event:  event,
	})
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	publishCtx, cancel := context.WithTimeout(ctx, realtimePublishTTL)
	defer cancel()
	return client.Publish(publishCtx, realtimeChannel, payload).Err()
}

func decodeRealtimePayload(raw string) (redisEventPayload, bool) {
	payload := redisEventPayload{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return payload, true
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, false
	}
	return payload, true
}

func runRealtimeSubscriber(client *redis.Client) {
	ctx := context.Background()
	pubsub := client.Subscribe(ctx, realtimeChannel)
	defer func() {
		_ = pubsub.Close()
		realtimeState.mu.Lock()
		realtimeState.started = false
		realtimeState.mu.Unlock()
	}()

	if _, err := pubsub.Receive(ctx); err != nil {
		if log.Logger != nil {
			log.Logger.Warn("订阅系统消息实时通道失败", zap.Error(err))
		}
		return
	}

	for {
		message, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if log.Logger != nil {
				log.Logger.Warn("系统消息实时订阅中断", zap.Error(err))
			}
			return
		}
		payload, ok := decodeRealtimePayload(message.Payload)
		if !ok || payload.Source == realtimeSourceID {
			continue
		}
		BroadcastEvent(payload.Event)
	}
}

func (h *hub) register(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.userID] == nil {
		h.clients[client.userID] = make(map[*wsClient]struct{})
	}
	h.clients[client.userID][client] = struct{}{}
}

func (h *hub) unregister(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[client.userID]
	if clients == nil {
		_ = client.conn.Close()
		return
	}
	delete(clients, client)
	if len(clients) == 0 {
		delete(h.clients, client.userID)
	}
	_ = client.conn.Close()
}

func (h *hub) broadcast(event Event) {
	h.broadcastToUser(event.UserID, event)
}

func (h *hub) broadcastToUser(userID uint, event Event) {
	if userID == 0 {
		return
	}
	h.mu.RLock()
	clients := h.clients[userID]
	list := make([]*wsClient, 0, len(clients))
	for client := range clients {
		list = append(list, client)
	}
	h.mu.RUnlock()

	for _, client := range list {
		if err := client.writeJSON(event); err != nil {
			h.unregister(client)
		}
	}
}

func (c *wsClient) writeJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(realtimeWriteTimeout))
	return c.conn.WriteJSON(v)
}

func (c *wsClient) keepalive() {
	ticker := time.NewTicker(realtimePingPeriod)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		_ = c.conn.SetWriteDeadline(time.Now().Add(realtimeWriteTimeout))
		err := c.conn.WriteMessage(websocket.PingMessage, nil)
		c.mu.Unlock()
		if err != nil {
			defaultHub.unregister(c)
			return
		}
	}
}

func buildRealtimeSourceID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return host + ":" + strconv.Itoa(os.Getpid())
}
