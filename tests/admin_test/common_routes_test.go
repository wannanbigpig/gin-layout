package admin_test

import (
	"net/http"
	"net/url"
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
}

func TestWebSocketEndpointRejectsMissingTicket(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/common/ws", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Equal(t, e.NotLogin, decodeResult(t, resp).Code)
}
