package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
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
	if data == nil {
		r.result.Data = emptyObject()
		return r
	}
	if isScalarData(data) {
		r.result.Data = &defaultRes{Result: data}
		return r
	}
	r.result.Data = data
	return r
}

// SetMessage 设置返回自定义错误消息
func (r *Response) SetMessage(message string) *Response {
	r.result.Msg = message
	return r
}

// json 返回 gin 框架的 JSON 响应
func (r *Response) json(c *gin.Context) {
	// 如果消息为空，使用错误码对应的默认消息
	if r.result.Msg == "" {
		r.result.Msg = errors.NewErrorText(config.GetConfig().Language).Text(r.result.Code)
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

func isScalarData(data any) bool {
	switch data.(type) {
	case bool, string,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64:
		return true
	default:
		return false
	}
}
