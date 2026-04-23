package response

import (
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
)

// Result API响应结果结构
type Result struct {
	Code      int    `json:"code"`
	Msg       string `json:"msg"`
	Data      any    `json:"data"`
	Cost      string `json:"cost"`
	RequestId string `json:"request_id"`
}

// NewResult 创建新的响应结果
func NewResult() *Result {
	return &Result{
		Code:      0,
		Msg:       "",
		Data:      emptyObject(),
		Cost:      "",
		RequestId: "",
	}
}

// Response 响应处理器
type Response struct {
	httpCode int
	result   *Result
	msgKey   string
	msgArgs  []any
}

// Resp 创建响应处理器实例
func Resp() *Response {
	return &Response{
		httpCode: http.StatusOK,
		result:   NewResult(),
	}
}

// Fail 错误返回
func (r *Response) Fail(c *gin.Context, code int, msg string, data ...any) {
	r.SetCode(code)
	r.SetMessage(msg)
	if len(data) > 0 && data[0] != nil {
		r.WithData(data[0])
	}
	r.json(c)
}

// FailCode 自定义错误码返回
func (r *Response) FailCode(c *gin.Context, code int, msg ...string) {
	r.SetCode(code)
	if len(msg) > 0 && msg[0] != "" {
		r.SetMessage(msg[0])
	}
	r.json(c)
}

// FailCodeByKey 自定义错误码返回（按文案 key 国际化）。
func (r *Response) FailCodeByKey(c *gin.Context, code int, key string, args ...any) {
	r.SetCode(code)
	r.SetMessageKey(key, args...)
	r.json(c)
}

// Success 正确返回
func (r *Response) Success(c *gin.Context) {
	r.SetCode(errors.SUCCESS)
	r.json(c)
}

// WithDataSuccess 成功后需要返回值
func (r *Response) WithDataSuccess(c *gin.Context, data interface{}) {
	r.SetCode(errors.SUCCESS)
	r.WithData(data)
	r.json(c)
}

// SetCode 设置返回code码
func (r *Response) SetCode(code int) *Response {
	r.result.Code = code
	return r
}

// SetHttpCode 设置http状态码
func (r *Response) SetHttpCode(code int) *Response {
	r.httpCode = code
	return r
}

// defaultRes 默认响应数据结构
type defaultRes struct {
	Result any `json:"result"`
}

// WithData 设置返回data数据
func (r *Response) WithData(data any) *Response {
	if isNilData(data) {
		r.result.Data = emptyObject()
		return r
	}
	if !isObjectData(data) {
		r.result.Data = &defaultRes{Result: data}
		return r
	}
	r.result.Data = data
	return r
}

// SetMessage 设置返回自定义错误消息
func (r *Response) SetMessage(message string) *Response {
	r.result.Msg = message
	r.msgKey = ""
	r.msgArgs = nil
	return r
}

// SetMessageKey 设置返回错误文案 key（供国际化解析）。
func (r *Response) SetMessageKey(key string, args ...any) *Response {
	r.msgKey = key
	r.msgArgs = append([]any(nil), args...)
	return r
}

// json 返回 gin 框架的 JSON 响应
func (r *Response) json(c *gin.Context) {
	// 如果消息为空，使用错误码对应的默认消息
	if r.result.Msg == "" {
		language := config.GetConfig().Language
		if c != nil {
			if locale, exists := c.Get(global.ContextKeyLocale); exists {
				if localeText, ok := locale.(string); ok {
					language = i18n.ToErrorLanguage(localeText)
				}
			}
		}
		errorText := errors.NewErrorText(language)
		if r.msgKey != "" {
			if msg, ok := errorText.TextByKey(r.msgKey, r.msgArgs...); ok && msg != "" {
				r.result.Msg = msg
			}
		}
		if r.result.Msg == "" {
			r.result.Msg = errorText.Text(r.result.Code)
		}
	}

	// 计算请求耗时
	r.result.Cost = time.Since(c.GetTime(global.ContextKeyRequestStartTime)).String()
	r.result.RequestId = c.GetString(global.ContextKeyRequestID)
	c.AbortWithStatusJSON(r.httpCode, r.result)
}

// Success 业务成功响应（便捷方法）
func Success(c *gin.Context, data ...any) {
	if len(data) > 0 && data[0] != nil {
		Resp().WithDataSuccess(c, data[0])
		return
	}
	Resp().Success(c)
}

// FailCode 业务失败响应（便捷方法）
func FailCode(c *gin.Context, code int, data ...any) {
	if len(data) > 0 && data[0] != nil {
		Resp().WithData(data[0]).FailCode(c, code)
		return
	}
	Resp().FailCode(c, code)
}

// FailCodeByKey 业务失败响应（按 key 解析多语言文案）。
func FailCodeByKey(c *gin.Context, code int, key string, args ...any) {
	Resp().FailCodeByKey(c, code, key, args...)
}

// Fail 业务失败响应（便捷方法）
func Fail(c *gin.Context, code int, message string, data ...any) {
	if len(data) > 0 && data[0] != nil {
		Resp().WithData(data[0]).Fail(c, code, message)
		return
	}
	Resp().Fail(c, code, message)
}

func emptyObject() map[string]any {
	return map[string]any{}
}

func isNilData(data any) bool {
	if data == nil {
		return true
	}

	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func isObjectData(data any) bool {
	value := reflect.ValueOf(data)

	// 解引用接口和指针，判断底层真实类型是否为对象形态。
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct, reflect.Map:
		return true
	default:
		return false
	}
}
