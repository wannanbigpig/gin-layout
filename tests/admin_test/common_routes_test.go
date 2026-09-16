package admin_test

import (
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	c "github.com/wannanbigpig/gin-layout/config"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestCommonUploadRequiresLogin(t *testing.T) {
	body := `{}`
	resp := anonymousPostRequest("/admin/v1/common/upload", &body)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestCommonUploadReachesMultipartValidationWithAuthorization(t *testing.T) {
	body := `{}`
	resp := postRequest("/admin/v1/common/upload", &body)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, e.InvalidParameter, decodeResult(t, resp).Code)
}

func TestWebSocketTicketAndEndpointFollowConfiguredRedis(t *testing.T) {
	body := `{}`
	ticketResp := postRequest("/admin/v1/common/ws-ticket", &body)
	ticketResult := decodeResult(t, ticketResp)
	if !c.GetConfig().Redis.Enable {
		assert.Equal(t, http.StatusServiceUnavailable, ticketResp.Code)
		assert.Equal(t, e.ServiceDependencyNotReady, ticketResult.Code)
		return
	}

	assert.Equal(t, http.StatusOK, ticketResp.Code)
	assert.Equal(t, e.SUCCESS, ticketResult.Code)
	data, ok := ticketResult.Data.(map[string]any)
	if !assert.True(t, ok) {
		return
	}
	ticket, _ := data["ticket"].(string)
	if !assert.NotEmpty(t, ticket) {
		return
	}

	wsResp := anonymousGetRequest("/admin/v1/common/ws", &url.Values{"ticket": {ticket}})
	assert.Equal(t, http.StatusBadRequest, wsResp.Code, "有效 ticket 应进入 WebSocket Upgrade 校验")
	reused := anonymousGetRequest("/admin/v1/common/ws", &url.Values{"ticket": {ticket}})
	assert.Equal(t, http.StatusUnauthorized, reused.Code, "已消费 ticket 不可重用")
}

func TestWebSocketTicketIsConsumedOnceConcurrently(t *testing.T) {
	body := `{}`
	response := postRequest("/admin/v1/common/ws-ticket", &body)
	if !c.GetConfig().Redis.Enable {
		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
		return
	}
	result := decodeResult(t, response)
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("ticket response: %s", response.Body.String())
	}
	ticket, ok := data["ticket"].(string)
	if !ok || ticket == "" {
		t.Fatal("missing ticket")
	}
	const clients = 8
	results := make(chan int, clients)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < clients; i++ {
		workers.Go(func() {
			<-start
			results <- anonymousGetRequest("/admin/v1/common/ws", &url.Values{"ticket": {ticket}}).Code
		})
	}
	close(start)
	workers.Wait()
	close(results)
	accepted := 0
	for status := range results {
		if status == http.StatusBadRequest {
			accepted++
		} else if status != http.StatusUnauthorized {
			t.Errorf("unexpected handshake status %d", status)
		}
	}
	if accepted != 1 {
		t.Fatalf("%d concurrent requests consumed the same ticket", accepted)
	}
}

func TestWebSocketEndpointRejectsMissingTicket(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/common/ws", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Equal(t, e.NotLogin, decodeResult(t, resp).Code)
}
