package validator

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	r "github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"reflect"
	"strings"
	"sync"
)

type Page struct {
	Page  float64 `form:"page" json:"page" binding:"min=1"`   // 必填，页面值>=1
	Limit float64 `form:"limit" json:"limit" binding:"min=1"` // 必填，每页条数值>=1
}

var trans ut.Translator // 全局验证器

var once sync.Once

func InitValidatorTrans(locale string) {
	once.Do(func() { validatorTrans(locale) })
}

func validatorTrans(locale string) {
	var v *validator.Validate
	var ok bool
	if v, ok = binding.Validator.Engine().(*validator.Validate); !ok {
		return
	}
	// 注册一个获取json tag的自定义方法
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		label := field.Tag.Get("label")
		if label == "" {
			label = field.Tag.Get("json")
			if label == "" {
				label = field.Tag.Get("form")
			}
		}

		if label == "-" {
			return ""
		}
		if label == "" {
			return field.Name
		}
		return label
	})

	zhT := zh.New() // 中文翻译器
	enT := en.New() // 英文翻译器
	uni := ut.New(enT, zhT, enT)

	// locale 通常取决于 http 请求头的 'Accept-Language'
	trans, ok = uni.GetTranslator(locale)
	if !ok {
		panic("Initialize a language not supported by the validator")
	}
	var err error
	// 注册翻译器
	switch locale {
	case "en":
		err = enTranslations.RegisterDefaultTranslations(v, trans)
	case "zh":
		err = zhTranslations.RegisterDefaultTranslations(v, trans)
	default:
		err = enTranslations.RegisterDefaultTranslations(v, trans)
	}
	if err != nil {
		panic("Failed to register translator when initializing validator")
	}
}

func ResponseError(c *gin.Context, err error) {
	if errs, ok := err.(validator.ValidationErrors); ok {
		fields := errs.Translate(trans)
		for _, err := range fields {
			r.Resp().FailCode(c, errors.InvalidParameter, err)
			break
		}
	} else {
		errStr := err.Error()
		// multipart:nextpart:eof 错误表示验证器需要一些参数，但是调用者没有提交任何参数
		if strings.ReplaceAll(strings.ToLower(errStr), " ", "") == "multipart:nextpart:eof" {
			r.Resp().FailCode(c, errors.InvalidParameter, "请根据要求填写必填项参数")
		} else {
			r.Resp().FailCode(c, errors.InvalidParameter, errStr)
		}
	}
}

func CheckQueryParams(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindQuery(obj); err != nil {
		ResponseError(c, err)
		return err
	}

	return nil
}

func CheckPostParams(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBind(obj); err != nil {
		ResponseError(c, err)
		return err
	}

	return nil
}
