package admin_test

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
	"github.com/wannanbigpig/gin-layout/tests"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

var (
	ts            *httptest.Server
	authorization string
)

func TestMain(m *testing.M) {
	ts = httptest.NewServer(tests.SetupRouter())
	now := time.Now()
	expiresAt := now.Add(time.Second * c.Config.Jwt.TTL)
	claims := token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{
			UserID:   1,
			Mobile:   "13200000000",
			Nickname: "admin",
		},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    global.Issuer, // 签发人
			//IssuedAt:  jwt.NewNumericDate(now),       // 签发时间
			Subject: global.Subject, // 签发主体
			//NotBefore: jwt.NewNumericDate(now),       // 生效时间
		},
	}
	accessToken, err := token.Generate(claims)
	authorization = fmt.Sprintf("%s%s", "Bearer ", accessToken)
	if err != nil {
		panic("创建管理员Token失败")
	}
	m.Run()
}

func postRequest(route string, body *string) *utils.HttpRequest {
	options := map[string]string{
		"Authorization": authorization,
	}
	return tests.Request("POST", route, body, options)
}

func getRequest(route string, queryParams *url.Values) *utils.HttpRequest {
	options := map[string]string{
		"Authorization": authorization,
	}
	return tests.GetRequest(route, queryParams, options)
}
