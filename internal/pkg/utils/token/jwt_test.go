package token

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
)

func TestGenerate(t *testing.T) {
	claims := jwt.MapClaims{
		"Id": 1,
	}
	_, err := Generate(claims)
	if err != nil {
		t.Error("生成Token失败")
	}
}

func TestParse(t *testing.T) {
	tokenString := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJZCI6MX0.JGVOAsonk7CoOaTS-b6dW86LLEOt8Z6kHhsFxIvqaCE"
	claims := jwt.MapClaims{}
	err := Parse(tokenString, claims)
	if err != nil {
		t.Error("解析Token失败")
	}
}

func TestGetAccessToken(t *testing.T) {
	authorization := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJZCI6MX0.JGVOAsonk7CoOaTS-b6dW86LLEOt8Z6kHhsFxIvqaCE"

	_, err := GetAccessToken(authorization)
	if err != nil {
		t.Error("获取Token失败")
	}
}
