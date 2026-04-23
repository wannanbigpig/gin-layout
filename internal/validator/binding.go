package validator

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	errcode "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	r "github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

const (
	eofErrorPattern         = `^multipart:nextpart:eof$`
	typeConvertErrorPattern = `parsing .*?: invalid syntax`
)

var (
	eofRegex         = regexp.MustCompile(eofErrorPattern)
	typeConvertRegex = regexp.MustCompile(typeConvertErrorPattern)
)

// ResponseError 处理错误并返回给前端。
func ResponseError(c *gin.Context, err error) {
	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		handleValidationError(c, errs)
	} else {
		handleBindingError(c, err)
	}
}

func handleValidationError(c *gin.Context, errs validator.ValidationErrors) {
	fields := errs.Translate(validatorRuntime.trans)
	for _, err := range fields {
		r.Resp().FailCode(c, errcode.InvalidParameter, err)
		return
	}
}

func handleBindingError(c *gin.Context, err error) {
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	switch {
	case errors.As(err, &typeErr):
		r.Resp().FailCode(c, errcode.InvalidParameter)
	case errors.As(err, &syntaxErr):
		r.Resp().FailCode(c, errcode.InvalidParameter)
	default:
		errStr := err.Error()
		switch {
		case isEOFError(errStr):
			r.Resp().FailCode(c, errcode.InvalidParameter)
		case isConvertError(errStr):
			r.Resp().FailCode(c, errcode.InvalidParameter)
		default:
			r.Resp().FailCode(c, errcode.InvalidParameter)
		}
	}
}

func isEOFError(errStr string) bool {
	if len(errStr) == 0 {
		return false
	}
	if errStr[0] == ' ' || errStr[len(errStr)-1] == ' ' {
		return eofRegex.MatchString(strings.TrimSpace(errStr))
	}
	return eofRegex.MatchString(errStr)
}

func isConvertError(errStr string) bool {
	if len(errStr) == 0 {
		return false
	}
	if errStr[0] == ' ' || errStr[len(errStr)-1] == ' ' {
		return typeConvertRegex.MatchString(strings.TrimSpace(errStr))
	}
	return typeConvertRegex.MatchString(errStr)
}

// CheckParams 检查请求参数。
func CheckParams(c *gin.Context, obj interface{}, bindFunc func(obj interface{}) error) error {
	if err := bindFunc(obj); err != nil {
		ResponseError(c, err)
		return err
	}
	return nil
}

// CheckQueryParams 检查 GET 请求的查询参数。
func CheckQueryParams(c *gin.Context, obj interface{}) error {
	return CheckParams(c, obj, c.ShouldBindQuery)
}

// CheckPostParams 检查 POST 请求的参数。
func CheckPostParams(c *gin.Context, obj interface{}) error {
	return CheckParams(c, obj, c.ShouldBind)
}
