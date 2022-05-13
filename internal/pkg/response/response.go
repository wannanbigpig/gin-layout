package response

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"l-admin.com/internal/pkg/error_code"
)

type result struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Cost    string      `json:"cost"`
}

type Response struct {
	httpCode int
	result   *result
}

// Fail 错误返回
func (r *Response) Fail(c *gin.Context) {
	r.SetCode(error_code.FAILURE)
	r.json(c)
}

// FailCode 自定义错误码返回
func (r *Response) FailCode(c *gin.Context, code int, msg ...string) {
	r.SetCode(code)
	if msg != nil {
		r.WithMessage(msg[0])
	}
	r.json(c)
}

// Success 正确返回
func (r *Response) Success(c *gin.Context) {
	r.SetCode(error_code.SUCCESS)
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

// WithData 设置返回data数据
func (r *Response) WithData(data interface{}) *Response {
	r.result.Data = data
	return r
}

// WithMessage 设置返回自定义错误消息
func (r *Response) WithMessage(message string) *Response {
	r.result.Message = message
	return r
}

// json 返回 gin 框架的 HandlerFunc
func (r *Response) json(c *gin.Context) {
	if r.result.Message == "" {
		r.result.Message = "unknown error"
		if msg, ok := error_code.Text(r.result.Code); ok == true {
			r.result.Message = msg
		}
	}
	// if r.Data == nil {
	// 	r.Data = struct{}{}
	// }

	r.result.Cost = fmt.Sprintf("%v", time.Since(c.GetTime("requestStartTime")))
	c.AbortWithStatusJSON(r.httpCode, r.result)
}

// NewResponse 构造一个 Response
func NewResponse() *Response {
	return &Response{
		httpCode: http.StatusOK,
		result: &result{
			Code:    0,
			Message: "",
			Data:    nil,
			Cost:    "",
		},
	}
}
