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
	"regexp"
	"strings"
	"sync"
)

var trans ut.Translator // 全局验证器

var once sync.Once

var validate *validator.Validate

func InitValidatorTrans(locale string) {
	once.Do(func() { validatorTrans(locale) })
}

func validatorTrans(locale string) {
	var ok bool
	if validate, ok = binding.Validator.Engine().(*validator.Validate); !ok {
		panic("Failed to initialize the validator")
	}
	// 注册自定义验证消息
	registerValidation()
	// 注册一个获取json tag的自定义方法
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
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

	trans, ok = uni.GetTranslator(locale)
	if !ok {
		panic("Initialize a language not supported by the validator")
	}
	var err error
	// 注册翻译器
	switch locale {
	case "en":
		err = enTranslations.RegisterDefaultTranslations(validate, trans)
	case "zh":
		err = zhTranslations.RegisterDefaultTranslations(validate, trans)
	default:
		err = enTranslations.RegisterDefaultTranslations(validate, trans)
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

func registerValidation() {
	// 注册手机号验证规则
	err := validate.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		return regexp.MustCompile(`^1[3456789]\d{9}$`).MatchString(fl.Field().String())
	})
	if err != nil {
		panic("Failed to register the mobile phone number verification rule")
	}
}
