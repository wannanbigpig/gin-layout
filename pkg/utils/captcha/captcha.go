package captcha

import (
	"strings"

	"github.com/mojocn/base64Captcha"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

type Item struct {
	Id     string `json:"id"`
	B64s   string `json:"b64s"`
	Answer string `json:"answer"`
}

var store base64Captcha.Store

// Generate 创建验证码
func Generate(driver base64Captcha.Driver, customStore *base64Captcha.Store) (item *Item, err error) {
	if customStore == nil {
		store = base64Captcha.DefaultMemStore
	} else {
		store = *customStore
	}

	c := base64Captcha.NewCaptcha(driver, store)

	id, b64s, answer, err := c.Generate()
	if err != nil {
		return nil, errors.NewBusinessError(1, "Failed to generate verification code")
	}

	if config.Config.AppEnv != "local" {
		answer = ""
	}

	return &Item{
		Id:     id,
		B64s:   b64s,
		Answer: answer,
	}, nil
}

// Verify 校验验证码
func Verify(id, value string) bool {
	// 如果 store 未初始化，使用默认的内存存储
	if store == nil {
		store = base64Captcha.DefaultMemStore
	}

	return strings.EqualFold(value, store.Get(id, true))
}
