package errors

import (
	"errors"
	"fmt"

	c "github.com/wannanbigpig/gin-layout/config"
)

// BusinessError 表示带业务码的可控错误。
type BusinessError struct {
	code       int
	message    string
	contextErr []error
}

// Error 实现 error 接口。
func (e *BusinessError) Error() string {
	return fmt.Sprintf("[Code]:%d [Msg]:%s, [context error] %s", e.code, e.message, e.contextErr)
}

// GetCode 返回业务错误码。
func (e *BusinessError) GetCode() int {
	return e.code
}

// GetMessage 返回业务错误消息。
func (e *BusinessError) GetMessage() string {
	return e.message
}

// SetCode 设置业务错误码。
func (e *BusinessError) SetCode(code int) {
	e.code = code
}

// SetMessage 设置业务错误消息。
func (e *BusinessError) SetMessage(message string) {
	e.message = message
}

// SetContextErr 追加底层上下文错误。
func (e *BusinessError) SetContextErr(err error) {
	e.contextErr = append(e.contextErr, err)
}

// GetContextErr 返回附带的上下文错误列表。
func (e *BusinessError) GetContextErr() []error {
	return e.contextErr
}

// NewBusinessError 创建业务错误。
func NewBusinessError(code int, message ...string) *BusinessError {
	var msg string
	if message != nil {
		msg = message[0]
	} else {
		msg = NewErrorText(c.GetConfig().Language).Text(code)
	}
	err := new(BusinessError)
	err.SetCode(code)
	err.SetMessage(msg)
	return err
}

// Error 提供错误转换辅助方法。
type Error struct{}

// AsBusinessError 尝试把任意错误转换为 BusinessError。
func (e *Error) AsBusinessError(err error) (*BusinessError, error) {
	var BusinessError = new(BusinessError)
	if errors.As(err, &BusinessError) {
		return BusinessError, nil
	}
	return nil, err
}
