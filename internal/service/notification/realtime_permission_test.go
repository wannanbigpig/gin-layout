package notification

import (
	"errors"
	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"testing"
	"time"
)

func TestMonitorChannelUsesServerPermissionsAndRechecksBeforePush(t *testing.T) {
	m, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act`)
	if err != nil {
		t.Fatal(err)
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	previous := loadChannelEnforcer
	loadChannelEnforcer = func() (*casbinx.CasbinEnforcer, error) { return &casbinx.CasbinEnforcer{Enforcer: enforcer}, nil }
	t.Cleanup(func() { loadChannelEnforcer = previous })
	client := &wsClient{userID: 20, claims: &token.AdminCustomClaims{UserID: 20}}
	handleClientMessage(client, []byte(`{"action":"subscribe","channel":"monitor"}`))
	if client.IsSubscribed("monitor") {
		t.Fatal("ordinary authenticated user subscribed without API permission")
	}
	subject := global.CasbinAdminUserPrefix + global.CasbinSeparator + "20"
	if _, err := enforcer.AddPolicy(subject, "/admin/v1/dashboard/monitor", "GET"); err != nil {
		t.Fatal(err)
	}
	handleClientMessage(client, []byte(`{"action":"subscribe","channel":"monitor"}`))
	if !client.IsSubscribed("monitor") {
		t.Fatal("authorized user could not subscribe")
	}
	serverConn, peerConn, cleanup := newRealtimeTestPair(t)
	defer cleanup()
	client.conn = serverConn
	h := &hub{clients: map[uint]map[*wsClient]struct{}{20: {client: {}}}}
	h.broadcastToChannel("monitor", GenericMessage{Type: "monitor"})
	_ = peerConn.SetReadDeadline(time.Now().Add(time.Second))
	var delivered GenericMessage
	if err := peerConn.ReadJSON(&delivered); err != nil || delivered.Type != "monitor" {
		t.Fatalf("authorized monitor delivery failed: %#v, %v", delivered, err)
	}
	client.conn = nil
	if _, err := enforcer.RemovePolicy(subject, "/admin/v1/dashboard/monitor", "GET"); err != nil {
		t.Fatal(err)
	}
	// A write to the nil connection would panic: revoked subscriptions must be removed before delivery.
	h.broadcastToChannel("monitor", GenericMessage{Type: "monitor"})
	if client.IsSubscribed("monitor") {
		t.Fatal("revoked permission retained subscription")
	}
	loadChannelEnforcer = func() (*casbinx.CasbinEnforcer, error) { return nil, errors.New("unavailable") }
	client.SubscribeChannel("monitor")
	if client.IsSubscribed("monitor") {
		t.Fatal("permission lookup failure must deny")
	}
	client.claims.IsSuperAdmin = global.Yes
	client.SubscribeChannel("monitor")
	if !client.IsSubscribed("monitor") {
		t.Fatal("super admin should retain HTTP-equivalent access")
	}
	client.SubscribeChannel("unknown")
	if client.IsSubscribed("unknown") {
		t.Fatal("unknown channel should be denied")
	}
}
