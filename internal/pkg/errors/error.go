package errors

import (
	"errors"
	"fmt"
	c "github.com/wannanbigpig/gin-layout/config"
)

type BusinessError struct {
	code       int
	message    string
	contextErr []error
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("[Code]:%d [Msg]:%s, [context error] %s", e.code, e.message, e.contextErr)
}

func (e *BusinessError) GetCode() int {
	return e.code
}

func (e *BusinessError) GetMessage() string {
	return e.message
}

func (e *BusinessError) SetCode(code int) {
	e.code = code
}

func (e *BusinessError) SetMessage(message string) {
	e.message = message
}

func (e *BusinessError) SetContextErr(err error) {
	e.contextErr = append(e.contextErr, err)
}

func (e *BusinessError) GetContextErr() []error {
	return e.contextErr
}

// NewBusinessError Create a business error
func NewBusinessError(code int, message ...string) *BusinessError {
	var msg string
	if message != nil {
		msg = message[0]
	} else {
		msg = NewErrorText(c.Config.Language).Text(code)
	}
	err := new(BusinessError)
	err.SetCode(code)
	err.SetMessage(msg)
	return err
}

type Error struct{}

func (e *Error) AsBusinessError(err error) (*BusinessError, error) {
	var BusinessError = new(BusinessError)
	if errors.As(err, &BusinessError) {
		return BusinessError, nil
	}
	return nil, err
}
