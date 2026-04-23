package errors

import (
	stderrors "errors"
	"fmt"
	"strings"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

// BusinessError 表示带业务码的可控错误。
type BusinessError struct {
	code            int
	message         string
	messageKey      string
	messageArgs     []any
	explicitMessage bool
	contextErrs     []error
}

// Error 实现 error 接口。
func (e *BusinessError) Error() string {
	if len(e.contextErrs) == 0 {
		return fmt.Sprintf("[Code]:%d [Msg]:%s", e.code, e.message)
	}
	msgs := make([]string, 0, len(e.contextErrs))
	for _, err := range e.contextErrs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("[Code]:%d [Msg]:%s, [context error] %s", e.code, e.message, strings.Join(msgs, "; "))
}

// GetCode 返回业务错误码。
func (e *BusinessError) GetCode() int {
	return e.code
}

// GetMessage 返回业务错误消息。
func (e *BusinessError) GetMessage() string {
	return e.message
}

// GetMessageKey 返回业务错误文案 key。
func (e *BusinessError) GetMessageKey() string {
	return e.messageKey
}

// GetMessageArgs 返回业务错误文案参数。
func (e *BusinessError) GetMessageArgs() []any {
	return e.messageArgs
}

// HasExplicitMessage 返回是否为业务代码调用方显式提供的消息文本。
func (e *BusinessError) HasExplicitMessage() bool {
	return e.explicitMessage
}

// HasMessageKey 返回是否携带文案 key。
func (e *BusinessError) HasMessageKey() bool {
	return strings.TrimSpace(e.messageKey) != ""
}

// SetCode 设置业务错误码。
func (e *BusinessError) SetCode(code int) {
	e.code = code
}

// SetMessage 设置业务错误消息。
func (e *BusinessError) SetMessage(message string) {
	e.message = message
	e.messageKey = ""
	e.messageArgs = nil
	e.explicitMessage = strings.TrimSpace(message) != ""
}

// AppendContextErr 追加底层上下文错误。
func (e *BusinessError) AppendContextErr(err error) {
	e.contextErrs = append(e.contextErrs, err)
}

// GetContextErr 返回附带的上下文错误列表。
func (e *BusinessError) GetContextErr() []error {
	return e.contextErrs
}

// NewBusinessError 创建业务错误。
func NewBusinessError(code int, message ...string) *BusinessError {
	msg := ""
	explicitMessage := false
	if len(message) > 0 && strings.TrimSpace(message[0]) != "" {
		msg = message[0]
		explicitMessage = true
	} else {
		msg = NewErrorText(c.GetConfig().Language).Text(code)
	}
	return &BusinessError{
		code:            code,
		message:         msg,
		explicitMessage: explicitMessage,
	}
}

// NewBusinessErrorWithKey 创建带文案 key 的业务错误。
func NewBusinessErrorWithKey(code int, messageKey string, messageArgs ...any) *BusinessError {
	msg := ""
	if key := strings.TrimSpace(messageKey); key != "" {
		if translated, ok := NewErrorText(c.GetConfig().Language).TextByKey(key, messageArgs...); ok {
			msg = translated
		}
	}
	if msg == "" {
		msg = NewErrorText(c.GetConfig().Language).Text(code)
	}

	return &BusinessError{
		code:            code,
		message:         msg,
		messageKey:      strings.TrimSpace(messageKey),
		messageArgs:     append([]any(nil), messageArgs...),
		explicitMessage: false,
	}
}

// Error 提供错误转换辅助方法。
type Error struct{}

// AsBusinessError 尝试把任意错误转换为 BusinessError。
func (e *Error) AsBusinessError(err error) (*BusinessError, error) {
	var be *BusinessError
	if stderrors.As(err, &be) {
		return be, nil
	}
	return nil, err
}

// NewDependencyNotReadyError 返回统一的依赖未就绪业务错误。
func NewDependencyNotReadyError(message ...string) *BusinessError {
	return NewBusinessError(ServiceDependencyNotReady, message...)
}

// IsDependencyNotReady 判断错误是否表示底层依赖尚未就绪。
func IsDependencyNotReady(err error) bool {
	if err == nil {
		return false
	}
	if stderrors.Is(err, model.ErrDBUninitialized) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "mysql not initialized")
}
