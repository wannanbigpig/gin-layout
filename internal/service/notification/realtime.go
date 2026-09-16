package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/wannanbigpig/gin-layout/internal/pkg/redisutil"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/corsutil"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

const (
	realtimeChannel       = "sys_messages:realtime"
	sessionControlChannel = "sys:session:control"
	realtimePublishTTL    = 3 * time.Second
	realtimeWriteTimeout  = 5 * time.Second
	realtimePingPeriod    = 30 * time.Second
	// realtimeRecheckEveryPings 心跳主动反查频率：每 N 次 ping（约 N×realtimePingPeriod）
	// 调用一次 auth.IsPrincipalValid，作为 Redis 强踢消息丢失与 token 过期的兜底层。
	realtimeRecheckEveryPings = 10

	// wsMaxConnsPerUser 每用户最大 WebSocket 连接数，超出时拒绝新连接。
	wsMaxConnsPerUser = 5
	// wsMaxConnsTotal 全局最大 WebSocket 连接数，超出时拒绝新连接。
	wsMaxConnsTotal = 5000
	// wsReadLimit 客户端单条消息最大字节数（1KB），超出时服务端主动断开。
	wsReadLimit = 1024
	// wsPongWait 客户端 pong 响应超时时间，超过未收到 pong 则断开连接。
	wsPongWait = 60 * time.Second
)

type redisEventPayload struct {
	Source string `json:"source"`
	Event  Event  `json:"event"`
}

// controlEnvelope 是会话强踢指令的跨实例传输信封，Source 用于跳过本实例自身发布的消息。
type controlEnvelope struct {
	Source  string              `json:"source"`
	Control auth.SessionControl `json:"control"`
}

type wsClient struct {
	userID    uint
	sessionID string
	claims    *token.AdminCustomClaims
	conn      *websocket.Conn
	mu        sync.Mutex
	closeOnce sync.Once
	subs      map[string]struct{} // 订阅的频道集合
	subMu     sync.RWMutex
}

type hub struct {
	mu              sync.RWMutex
	clients         map[uint]map[*wsClient]struct{}
	totalConns      int
	shuttingDown    bool
	maxConnsPerUser int
	maxConnsTotal   int
}

// wsOriginChecker 可在测试中替换为始终允许的实现。
var wsOriginChecker = checkWebSocketOrigin

var loadChannelEnforcer = casbinx.GetEnforcer

var (
	defaultHub = &hub{
		clients:         make(map[uint]map[*wsClient]struct{}),
		maxConnsPerUser: wsMaxConnsPerUser,
		maxConnsTotal:   wsMaxConnsTotal,
	}
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return wsOriginChecker(r) },
	}
	realtimeSourceID = uuid.New().String()
	realtimeRunner   = func(ctx context.Context, _ *redis.Client) {
		runRealtimeSubscriberWithSource(ctx, data.RedisClient, time.Second)
	}
	realtimeState = struct {
		mu      sync.Mutex
		started bool
		cancel  context.CancelFunc
		done    chan struct{}
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

	startRealtimeSubscriber(client)
}

func startRealtimeSubscriber(client *redis.Client) bool {
	if client == nil {
		return false
	}
	realtimeState.mu.Lock()
	if realtimeState.started {
		realtimeState.mu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	realtimeState.started = true
	realtimeState.cancel = cancel
	realtimeState.done = done
	realtimeState.mu.Unlock()

	go func() {
		defer close(done)
		defer func() {
			realtimeState.mu.Lock()
			if realtimeState.done == done {
				realtimeState.started = false
				realtimeState.cancel = nil
				realtimeState.done = nil
			}
			realtimeState.mu.Unlock()
		}()
		realtimeRunner(ctx, client)
	}()
	return true
}

func StopRealtimeSubscriber(timeout time.Duration) {
	realtimeState.mu.Lock()
	cancel := realtimeState.cancel
	done := realtimeState.done
	realtimeState.mu.Unlock()

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

// ServeWebsocket 升级请求为 WebSocket 并注册到 hub。
// 调用方（控制器）需保证 claims 非 nil 且 UserID/jti 已校验。
func ServeWebsocket(w http.ResponseWriter, r *http.Request, claims *token.AdminCustomClaims) error {
	EnsureRealtimeDispatcherStarted()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	// 限制客户端单条消息大小，超出时服务端主动断开连接。
	conn.SetReadLimit(wsReadLimit)

	client := &wsClient{
		userID:    claims.UserID,
		sessionID: claims.ID,
		claims:    claims,
		conn:      conn,
	}
	if err := defaultHub.register(client); err != nil {
		closeMsg := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error())
		_ = conn.WriteMessage(websocket.CloseMessage, closeMsg)
		_ = conn.Close()
		return nil
	}

	// 设置 pong 超时：客户端必须在 wsPongWait 内回复 pong，否则连接将被 ReadMessage 错误触发关闭。
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	go client.keepalive()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			defaultHub.unregister(client)
			return nil
		}
		// 处理客户端的订阅/退订消息
		handleClientMessage(client, message)
	}
}

// clientMessage 客户端发送的消息格式。
type clientMessage struct {
	Action  string `json:"action"`
	Channel string `json:"channel"`
}

// handleClientMessage 处理客户端的订阅/退订消息。
func handleClientMessage(client *wsClient, message []byte) {
	var msg clientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}
	switch msg.Action {
	case "ping":
		if err := client.writeJSON(GenericMessage{Type: "pong"}); err != nil {
			defaultHub.unregister(client)
		}
	case "subscribe":
		client.SubscribeChannel(msg.Channel)
	case "unsubscribe":
		client.UnsubscribeChannel(msg.Channel)
	}
}

func BroadcastEvent(event Event) {
	defaultHub.broadcastToUser(event.UserID, event)
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
	return client.Publish(publishCtx, redisutil.Channel(realtimeChannel), payload).Err()
}

// PublishSessionControl 派发会话强踢指令：先在本实例立即执行，再按 Redis 开关跨实例广播。
// 由 auth.SessionControlPublisher 注入调用，是 auth → notification 的唯一反向入口。
func PublishSessionControl(ctx context.Context, ctrl auth.SessionControl) {
	if ctrl.UserID == 0 {
		return
	}
	defaultHub.kick(ctrl)

	cfg := config.GetConfig()
	if cfg == nil || !cfg.Redis.Enable {
		return
	}
	client := data.RedisClient()
	if client == nil {
		return
	}
	payload, err := json.Marshal(controlEnvelope{
		Source:  realtimeSourceID,
		Control: ctrl,
	})
	if err != nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	publishCtx, cancel := context.WithTimeout(ctx, realtimePublishTTL)
	defer cancel()
	if err := client.Publish(publishCtx, redisutil.Channel(sessionControlChannel), payload).Err(); err != nil && log.Logger != nil {
		log.Logger.Warn("发布会话强踢指令失败，跨实例强踢降级为心跳反查兜底",
			zap.Error(err), zap.Uint("user_id", ctrl.UserID), zap.String("session_id", ctrl.SessionID))
	}
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

func decodeControlEnvelope(raw string) (controlEnvelope, bool) {
	envelope := controlEnvelope{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return envelope, false
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return envelope, false
	}
	return envelope, true
}

func runRealtimeSubscriber(ctx context.Context, client *redis.Client) {
	runRealtimeSubscriberWithSource(ctx, func() *redis.Client { return client }, time.Second)
}

func runRealtimeSubscriberWithSource(ctx context.Context, clientSource func() *redis.Client, refreshEvery time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	var subscription redisutil.DynamicSubscription
	defer subscription.Close()
	refresh := func() <-chan *redis.Message {
		return subscription.Refresh(ctx, clientSource(), redisutil.Channel(realtimeChannel), redisutil.Channel(sessionControlChannel))
	}
	messages := refresh()
	ticker := time.NewTicker(refreshEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			messages = refresh()
		case message, ok := <-messages:
			if !ok {
				subscription.Reset()
				messages = nil
				continue
			}
			if message != nil {
				dispatchRealtimeMessage(message.Channel, message.Payload)
			}
		}
	}
}

// dispatchRealtimeMessage 按订阅频道分发消息：实时事件走广播，会话控制走本地强踢。
func dispatchRealtimeMessage(channel, raw string) {
	switch channel {
	case redisutil.Channel(sessionControlChannel):
		envelope, ok := decodeControlEnvelope(raw)
		if !ok || envelope.Source == realtimeSourceID {
			return
		}
		defaultHub.kick(envelope.Control)
	case redisutil.Channel(realtimeChannel):
		payload, ok := decodeRealtimePayload(raw)
		if !ok || payload.Source == realtimeSourceID {
			return
		}
		BroadcastEvent(payload.Event)
	}
}

// GenericMessage 通用 WS 消息格式，支持多种频道。
type GenericMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// canAccessChannel 使用与 HTTP 监控接口相同的服务端权限，每次推送重新检查策略。
func (c *wsClient) canAccessChannel(channel string) bool {
	if channel != "monitor" || c.claims == nil || c.userID == 0 {
		return false
	}
	if c.claims.ExpiresAt != nil && !c.claims.ExpiresAt.Time.After(time.Now()) {
		return false
	}
	if c.claims.IsSuperAdmin == global.Yes || c.userID == global.SuperAdminId {
		return true
	}
	enforcer, err := loadChannelEnforcer()
	if err != nil || enforcer == nil {
		return false
	}
	userKey := fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, c.userID)
	allowed, err := enforcer.Enforce(userKey, "/admin/v1/dashboard/monitor", http.MethodGet)
	return err == nil && allowed
}

// SubscribeChannel 客户端订阅指定频道。
func (c *wsClient) SubscribeChannel(channel string) {
	if !c.canAccessChannel(channel) {
		return
	}
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if c.subs == nil {
		c.subs = make(map[string]struct{})
	}
	c.subs[channel] = struct{}{}
}

// UnsubscribeChannel 客户端退订指定频道。
func (c *wsClient) UnsubscribeChannel(channel string) {
	if channel == "" {
		return
	}
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if c.subs != nil {
		delete(c.subs, channel)
	}
}

// IsSubscribed 检查客户端是否订阅了指定频道。
func (c *wsClient) IsSubscribed(channel string) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	if c.subs == nil {
		return false
	}
	_, ok := c.subs[channel]
	return ok
}

// BroadcastToChannel 广播消息到指定频道的所有订阅者。
func BroadcastToChannel(channel string, msg GenericMessage) {
	defaultHub.broadcastToChannel(channel, msg)
}

// ChannelSubscriberCount 返回指定频道当前订阅连接数。
func ChannelSubscriberCount(channel string) int {
	return defaultHub.channelSubscriberCount(channel)
}

func (h *hub) broadcastToChannel(channel string, msg GenericMessage) {
	h.mu.RLock()
	var list []*wsClient
	for _, clients := range h.clients {
		for client := range clients {
			if client.IsSubscribed(channel) {
				list = append(list, client)
			}
		}
	}
	h.mu.RUnlock()

	for _, client := range list {
		if !client.canAccessChannel(channel) {
			client.UnsubscribeChannel(channel)
			continue
		}
		if err := client.writeJSON(msg); err != nil {
			h.unregister(client)
		}
	}
}

func (h *hub) channelSubscriberCount(channel string) int {
	if channel == "" {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, clients := range h.clients {
		for client := range clients {
			if client.IsSubscribed(channel) {
				count++
			}
		}
	}
	return count
}

// BroadcastToAll 广播消息到所有连接的客户端。
func BroadcastToAll(msg GenericMessage) {
	defaultHub.broadcastToAll(msg)
}

func (h *hub) broadcastToAll(msg GenericMessage) {
	h.mu.RLock()
	var list []*wsClient
	for _, clients := range h.clients {
		for client := range clients {
			list = append(list, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range list {
		if err := client.writeJSON(msg); err != nil {
			h.unregister(client)
		}
	}
}

func (h *hub) register(client *wsClient) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.shuttingDown {
		return errors.New("server shutting down")
	}

	// 全局连接数限制
	if h.totalConns >= h.maxConnsTotal {
		return errors.New("max total connections reached")
	}

	// 每用户连接数限制
	userConns := h.clients[client.userID]
	if len(userConns) >= h.maxConnsPerUser {
		return errors.New("max connections per user reached")
	}

	if h.clients[client.userID] == nil {
		h.clients[client.userID] = make(map[*wsClient]struct{})
	}
	h.clients[client.userID][client] = struct{}{}
	h.totalConns++
	return nil
}

func (h *hub) unregister(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[client.userID]
	if clients != nil {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			h.totalConns--
			if len(clients) == 0 {
				delete(h.clients, client.userID)
			}
		}
	}
	// 用 sync.Once 保护连接关闭，避免多路径并发调用导致 close-of-closed
	client.closeOnce.Do(func() {
		_ = client.conn.Close()
	})
}

// OnlineCount 返回当前 WebSocket 在线连接数（只读，用于监控面板）。
func OnlineCount() int {
	return defaultHub.onlineCount()
}

func (h *hub) onlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
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

// kick 强制下线指定用户的连接。ctrl.SessionID 为空时踢掉该用户全部连接，
// 非空时仅踢匹配 sessionID 的连接（支持多端精准强踢）。
func (h *hub) kick(ctrl auth.SessionControl) {
	if ctrl.UserID == 0 {
		return
	}
	h.mu.RLock()
	clients := h.clients[ctrl.UserID]
	list := make([]*wsClient, 0, len(clients))
	for client := range clients {
		if ctrl.SessionID == "" || client.sessionID == ctrl.SessionID {
			list = append(list, client)
		}
	}
	h.mu.RUnlock()

	code := ctrl.Code
	if code == 0 {
		code = auth.SessionControlCodeKicked
	}
	for _, client := range list {
		client.closeWithCode(code, ctrl.Reason)
		h.unregister(client)
	}
}

func (c *wsClient) writeJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(realtimeWriteTimeout))
	return c.conn.WriteJSON(v)
}

// closeWithCode 向客户端发送带业务 code 的 Close 帧（如 4001 会话失效 / 4002 强制下线）。
func (c *wsClient) closeWithCode(code int, reason string) {
	closeMsg := websocket.FormatCloseMessage(code, reason)
	c.mu.Lock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(realtimeWriteTimeout))
	_ = c.conn.WriteMessage(websocket.CloseMessage, closeMsg)
	c.mu.Unlock()
	c.closeOnce.Do(func() { _ = c.conn.Close() })
}

func (c *wsClient) keepalive() {

	ticker := time.NewTicker(realtimePingPeriod)
	defer ticker.Stop()
	pings := 0
	for range ticker.C {
		if c.claims != nil && c.claims.ExpiresAt != nil && !c.claims.ExpiresAt.Time.After(time.Now()) {
			c.closeWithCode(auth.SessionControlCodeExpired, "session expired")
			defaultHub.unregister(c)
			return
		}
		// 周期性主动反查会话有效性：作为 Redis 强踢消息丢失与 token 过期的兜底层。
		pings++
		if pings >= realtimeRecheckEveryPings {
			pings = 0
			if !c.sessionStillValid() {
				c.closeWithCode(auth.SessionControlCodeExpired, "session expired")
				defaultHub.unregister(c)
				return
			}
		}

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

// sessionStillValid 反查当前连接对应的会话是否仍有效：先判 token 是否过期，再查黑名单/撤销状态。
func (c *wsClient) sessionStillValid() bool {
	if c.claims == nil {
		return false
	}
	if exp, err := c.claims.GetExpirationTime(); err != nil || exp == nil || !exp.Time.After(time.Now()) {
		return false
	}
	return auth.NewLoginService().IsPrincipalValid(c.claims)
}

// checkWebSocketOrigin 校验 WebSocket 请求的 Origin 是否在 CORS 白名单内。
// 无 Origin 头或白名单为空时拒绝，防止跨站 WebSocket 劫持（CSWSH）。
func checkWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	cfg := config.GetConfig()
	if cfg == nil || len(cfg.CorsOrigins) == 0 {
		return false
	}
	// WebSocket 与 HTTP CORS 共用同一份 Origin 匹配实现，避免两套通配语义漂移；
	// allowWildcardAll=false：WS 握手必带身份，字面 "*" 不放行任意 Origin，杜绝 CSWSH。
	return corsutil.IsOriginAllowed(origin, cfg.CorsOrigins, false)
}

// ShutdownRealtimeDispatcher 主动关闭所有活跃的 WebSocket 连接，发送 Close 帧。
// 所有连接共享同一截止时间，避免停服耗时随连接数量线性增长。
func ShutdownRealtimeDispatcher(timeout ...time.Duration) bool {
	defaultHub.mu.Lock()
	if defaultHub.shuttingDown {
		defaultHub.mu.Unlock()
		return true
	}
	// 与 register 共用锁，覆盖 Upgrade 已 Hijack、尚未加入连接池的窗口。
	defaultHub.shuttingDown = true
	clients := defaultHub.clients
	defaultHub.clients = make(map[uint]map[*wsClient]struct{})
	defaultHub.totalConns = 0
	defaultHub.mu.Unlock()

	clientsList := make([]*wsClient, 0)
	for _, userClients := range clients {
		for client := range userClients {
			clientsList = append(clientsList, client)
		}
	}
	shutdownTimeout := realtimeWriteTimeout
	if len(timeout) > 0 && timeout[0] > 0 {
		shutdownTimeout = timeout[0]
	}
	closeMsg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down")
	deadline := time.Now().Add(shutdownTimeout)
	completed := closeRealtimeClientsWithin(clientsList, shutdownTimeout, func(client *wsClient) {
		// Gorilla 允许 WriteControl 与其他读写方法并发；无需等待普通写锁。
		_ = client.conn.WriteControl(websocket.CloseMessage, closeMsg, deadline)
		client.closeOnce.Do(func() { _ = client.conn.Close() })
	})
	if !completed {
		for _, client := range clientsList {
			client.closeOnce.Do(func() { _ = client.conn.Close() })
		}
	}
	return completed
}

func closeRealtimeClientsWithin(clients []*wsClient, timeout time.Duration, closeClient func(*wsClient)) bool {
	if len(clients) == 0 {
		return true
	}
	var wg sync.WaitGroup
	wg.Add(len(clients))
	for _, client := range clients {
		go func() {
			defer wg.Done()
			closeClient(client)
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	if timeout <= 0 {
		<-done
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
