package admin_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

func TestSessionListAndRevokeFlowWithConfiguredMySQL(t *testing.T) {
	requireWritableDB(t)

	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}
	marker := uniqueTestName("session")
	jwtID := uniqueCompactTestName("sessionjwt")
	defer func() {
		if err := db.Unscoped().Where("user_agent = ?", marker).Delete(&model.AdminLoginLogs{}).Error; err != nil {
			t.Errorf("清理会话测试数据失败: %v", err)
		}
		if redisClient := data.RedisClient(); redisClient != nil {
			if err := redisClient.Del(t.Context(), "blacklist:"+jwtID).Err(); err != nil {
				t.Errorf("清理会话测试 Redis 黑名单失败: %v", err)
			}
		}
	}()

	session := &model.AdminLoginLogs{
		UID:          1,
		Username:     "super_admin",
		JwtID:        jwtID,
		UserAgent:    marker,
		LoginStatus:  model.LoginStatusSuccess,
		Type:         model.LoginTypeLogin,
		IsRevoked:    model.IsRevokedNo,
		TokenExpires: &utils.FormatDate{Time: time.Now().Add(time.Hour)},
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("创建在线会话前置数据失败: %v", err)
	}

	listResp := getRequest("/admin/v1/auth/session/list", &url.Values{
		"username": {"super_admin"},
		"page":     {"1"},
		"per_page": {"20"},
	})
	assert.Equal(t, http.StatusOK, listResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, listResp).Code)

	revokePayload := mustJSONBody(t, map[string]any{
		"id":     session.ID,
		"reason": "接口测试强制下线",
	})
	revokeResp := postRequest("/admin/v1/auth/session/revoke", &revokePayload)
	assert.Equal(t, http.StatusOK, revokeResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, revokeResp).Code)

	if err := db.First(session, session.ID).Error; err != nil {
		t.Fatalf("读取撤销后的会话失败: %v", err)
	}
	assert.Equal(t, model.IsRevokedYes, session.IsRevoked)
	assert.Equal(t, model.RevokedCodeSystemForce, session.RevokedCode)
	assert.NotEmpty(t, session.RevokedReason)
}
