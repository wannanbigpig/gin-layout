package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
)

const acceptLanguageHeader = "Accept-Language"

// RequestLocaleHandler 解析请求语言并写入上下文。
func RequestLocaleHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		locale := i18n.ParseAcceptLanguage(c.GetHeader(acceptLanguageHeader))
		c.Set(global.ContextKeyLocale, locale)
		c.Next()
	}
}

// LocaleFromContext 从请求上下文中读取归一化语言。
func LocaleFromContext(c *gin.Context) string {
	if c == nil {
		return i18n.DefaultLocale
	}
	if locale, exists := c.Get(global.ContextKeyLocale); exists {
		if localeText, ok := locale.(string); ok {
			return i18n.NormalizeLocale(localeText)
		}
	}
	return i18n.DefaultLocale
}
