package validator

import (
	"errors"

	"github.com/gin-gonic/gin"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"

	errcode "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	r "github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

// ResponseError 处理错误并返回给前端。
func ResponseError(c *gin.Context, err error) {
	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		handleValidationError(c, errs)
	} else {
		r.Resp().FailCode(c, errcode.InvalidParameter)
	}
}

func handleValidationError(c *gin.Context, errs validator.ValidationErrors) {
	primary := validatorRuntime.translatorForRequest(c)
	fallback := validatorRuntime.fallbackTranslator(primary)
	for _, fieldErr := range errs {
		message := translateFieldError(fieldErr, primary, fallback)
		r.Resp().FailCode(c, errcode.InvalidParameter, message)
		return
	}
}

func translateFieldError(fieldErr validator.FieldError, primary, fallback ut.Translator) string {
	if primary != nil {
		if translated := fieldErr.Translate(primary); translated != "" && translated != fieldErr.Error() {
			return translated
		}
	}
	if fallback != nil {
		if translated := fieldErr.Translate(fallback); translated != "" && translated != fieldErr.Error() {
			return translated
		}
	}
	return fieldErr.Error()
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
