package validator

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"github.com/wannanbigpig/gin-layout/internal/pkg/error_code"
	response2 "github.com/wannanbigpig/gin-layout/pkg/response"
	"reflect"
	"strings"
)

type Page struct {
	Page  float64 `form:"page" json:"page" binding:"min=1"`   // 必填，页面值>=1
	Limit float64 `form:"limit" json:"limit" binding:"min=1"` // 必填，每页条数值>=1
}

var Trans ut.Translator // 全局验证器

func InitValidatorTrans(locale string) (err error) {
	// 修改gin框架中的Validator引擎属性，实现自定制
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// 注册一个获取json tag的自定义方法
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
		zhT := zh.New() // 中文翻译器
		enT := en.New() // 英文翻译器
		uni := ut.New(enT, zhT, enT)

		// locale 通常取决于 http 请求头的 'Accept-Language'
		var ok bool
		// 也可以使用 uni.FindTranslator(...) 传入多个locale进行查找
		Trans, ok = uni.GetTranslator(locale)
		if !ok {
			return fmt.Errorf("uni.GetTranslator(%s) failed", locale)
		}

		// 注册翻译器
		switch locale {
		case "en":
			err = enTranslations.RegisterDefaultTranslations(v, Trans)
		case "zh":
			err = zhTranslations.RegisterDefaultTranslations(v, Trans)
		default:
			err = enTranslations.RegisterDefaultTranslations(v, Trans)
		}
		return
	}
	return
}

func ResponseError(c *gin.Context, err error) {
	if errs, ok := err.(validator.ValidationErrors); ok {
		fields := errs.Translate(Trans)
		for _, err := range fields {
			response2.NewResponse().FailCode(c, error_code.ParamBindError, err)
			break
		}
	} else {
		errStr := err.Error()
		// multipart:nextpart:eof 错误表示验证器需要一些参数，但是调用者没有提交任何参数
		if strings.ReplaceAll(strings.ToLower(errStr), " ", "") == "multipart:nextpart:eof" {
			response2.NewResponse().FailCode(c, error_code.ParamBindError, "请根据要求填写必填项参数")
		} else {
			response2.NewResponse().FailCode(c, error_code.ParamBindError, errStr)
		}
	}
}

func CheckQueryParams(c *gin.Context, obj interface{}) error {
	//1.先按照验证器提供的基本语法，基本可以校验90%以上的不合格参数
	if err := c.ShouldBindQuery(obj); err != nil {
		// 将表单参数验证器出现的错误直接交给错误翻译器统一处理即可
		ResponseError(c, err)
		//response2.NewResponse().FailCode(c, error_code.ParamBindError, err.Error())
		return err
	}

	return nil
}

func CheckPostParams(c *gin.Context, obj interface{}) error {
	//1.先按照验证器提供的基本语法，基本可以校验90%以上的不合格参数
	if err := c.ShouldBind(obj); err != nil {
		// 将表单参数验证器出现的错误直接交给错误翻译器统一处理即可
		ResponseError(c, err)
		//response2.NewResponse().FailCode(c, error_code.ParamBindError, err.Error())
		return err
	}

	return nil
}
