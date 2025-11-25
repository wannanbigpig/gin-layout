package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"go.uber.org/zap"

	errcode "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	r "github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

var (
	trans    ut.Translator // 全局验证器
	once     sync.Once
	validate *validator.Validate

	// 预编译的正则表达式，避免每次验证时重新编译
	phoneNumberRegex = regexp.MustCompile(`^1[3456789]\d{9}$`)

	// 正则表达式缓存，用于 RegexpValidator
	regexCache sync.Map // map[string]*regexp.Regexp
)

// InitValidatorTrans 初始化验证器和翻译器
func InitValidatorTrans(locale string) {
	once.Do(func() { validatorTrans(locale) })
}

func validatorTrans(locale string) {
	var ok bool
	if validate, ok = getValidatorEngine(); !ok {
		panic("Failed to initialize the validator")
	}

	// 注册自定义验证规则
	initCustomRules(validate)

	// 注册获取 JSON 标签的自定义方法
	registerTagNameFunc(validate)

	// 注册翻译器
	initTranslator(validate, locale)
}

// 获取验证器引擎
func getValidatorEngine() (*validator.Validate, bool) {
	engine := binding.Validator.Engine()
	if engine == nil {
		return nil, false
	}
	validate, ok := engine.(*validator.Validate)
	return validate, ok
}

// registerTagNameFunc 注册获取 JSON 标签的自定义方法
func registerTagNameFunc(validate *validator.Validate) {
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		// 优化：减少字符串分配，直接返回找到的第一个有效标签
		if label := field.Tag.Get("label"); label != "" && label != "-" {
			return label
		}
		if json := field.Tag.Get("json"); json != "" && json != "-" {
			return json
		}
		if form := field.Tag.Get("form"); form != "" && form != "-" {
			return form
		}
		return field.Name
	})
}

// RegexpValidator 通用正则表达式验证器
// 使用缓存避免重复编译相同的正则表达式
func RegexpValidator(fl validator.FieldLevel) bool {
	param := fl.Param()
	if param == "" {
		return false
	}

	value := fl.Field().String()

	// 从缓存中获取已编译的正则表达式
	if cached, ok := regexCache.Load(param); ok {
		return cached.(*regexp.Regexp).MatchString(value)
	}

	// 编译正则表达式并缓存
	reg, err := regexp.Compile(param)
	if err != nil {
		return false
	}
	regexCache.Store(param, reg)
	return reg.MatchString(value)
}

// initCustomRules 注册自定义验证规则
func initCustomRules(validate *validator.Validate) {
	// 注册手机号规则（使用预编译的正则表达式）
	err := validate.RegisterValidation("phone_number", func(fl validator.FieldLevel) bool {
		return phoneNumberRegex.MatchString(fl.Field().String())
	})
	if err != nil {
		panic("registration of phone_number rule failed")
	}

	// 注册 required_if_exist 规则
	err = validate.RegisterValidation("required_if_exist", requiredIf)
	if err != nil {
		panic("registration of required_if_exist rule failed")
	}

	// 注册通用正则验证规则
	err = validate.RegisterValidation("regexp", RegexpValidator)
	if err != nil {
		panic("registration of regexp rule failed")
	}
}

// initTranslator 初始化语言翻译器
func initTranslator(validate *validator.Validate, locale string) {
	zhT := zh.New() // 中文翻译器
	enT := en.New() // 英文翻译器
	uni := ut.New(enT, zhT, enT)

	var ok bool
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

	// 注册自定义翻译
	if err := customRegisTranslation(); err != nil {
		panic("Failed to register custom translations")
	}
}

// ResponseError 处理错误并返回给前端
func ResponseError(c *gin.Context, err error) {
	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		handleValidationError(c, errs)
	} else {
		handleBindingError(c, err)
	}
}

// handleValidationError 处理验证错误
// 优化：只处理第一个错误，提前返回
func handleValidationError(c *gin.Context, errs validator.ValidationErrors) {
	fields := errs.Translate(trans)
	// fields 是 map[string]string 类型，遍历获取第一个错误
	for _, err := range fields {
		r.Resp().FailCode(c, errcode.InvalidParameter, err)
		return
	}
}

const (
	eofErrorPattern            = `^multipart:nextpart:eof$`
	typeConvertErrorPattern    = `parsing .*?: invalid syntax`
	errorMessageRequiredParams = "请根据要求填写必填项参数"
	errorMessageTypeConvert    = "参数类型错误，请传入正确格式的数据"
)

var (
	eofRegex         = regexp.MustCompile(eofErrorPattern)
	typeConvertRegex = regexp.MustCompile(typeConvertErrorPattern)
)

func handleBindingError(c *gin.Context, err error) {
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	switch {
	case errors.As(err, &typeErr):
		// JSON 结构体字段类型错误
		errMsg := fmt.Sprintf("%s 应该是 %s 类型，传入的是 %s 类型", typeErr.Field, typeErr.Type.Name(), reflect.TypeOf(typeErr.Value).Name())
		r.Resp().FailCode(c, errcode.InvalidParameter, errMsg)
	case errors.As(err, &syntaxErr):
		// JSON 语法错误
		r.Resp().FailCode(c, errcode.InvalidParameter, "请求参数解析失败：请检查 JSON 格式是否正确")
	default:
		errStr := err.Error()
		switch {
		case isEOFError(errStr):

			r.Resp().FailCode(c, errcode.InvalidParameter, errorMessageRequiredParams)
		case isConvertError(errStr):
			r.Resp().FailCode(c, errcode.InvalidParameter, errorMessageTypeConvert)
		default:

			r.Resp().FailCode(c, errcode.InvalidParameter, errStr)
		}
	}
}

// 判断是否为 EOF 错误，提升匹配逻辑的健壮性
// 优化：避免不必要的字符串分配
func isEOFError(errStr string) bool {
	// 快速检查：如果字符串为空或太长，直接返回 false
	if len(errStr) == 0 || len(errStr) > 30 {
		return false
	}
	// 只对可能包含空格的情况进行 TrimSpace
	if len(errStr) > 0 && (errStr[0] == ' ' || errStr[len(errStr)-1] == ' ') {
		return eofRegex.MatchString(strings.TrimSpace(errStr))
	}
	return eofRegex.MatchString(errStr)
}

func isConvertError(errStr string) bool {
	// 快速检查：如果字符串为空或很短，直接返回 false
	if len(errStr) == 0 {
		return false
	}
	// 只对可能包含空格的情况进行 TrimSpace
	if len(errStr) > 0 && (errStr[0] == ' ' || errStr[len(errStr)-1] == ' ') {
		return typeConvertRegex.MatchString(strings.TrimSpace(errStr))
	}
	return typeConvertRegex.MatchString(errStr)
}

// CheckParams 检查请求参数
func CheckParams(c *gin.Context, obj interface{}, bindFunc func(obj interface{}) error) error {
	if err := bindFunc(obj); err != nil {
		ResponseError(c, err)
		return err
	}
	return nil
}

// CheckQueryParams 检查GET请求的查询参数
func CheckQueryParams(c *gin.Context, obj interface{}) error {
	if err := CheckParams(c, obj, c.ShouldBindQuery); err != nil {
		return err
	}

	return nil
}

// CheckPostParams 检查POST请求的参数
func CheckPostParams(c *gin.Context, obj interface{}) error {
	if err := CheckParams(c, obj, c.ShouldBind); err != nil {
		return err
	}

	return nil
}

// requiredIf 字段B存在时，字段A必填
// 优化：减少字符串分配和类型转换
func requiredIf(fl validator.FieldLevel) bool {
	param := fl.Param()
	if param == "" {
		return false
	}

	// 优化：手动分割字符串，避免 strings.Fields 的额外分配
	params := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(param); i++ {
		if param[i] == ' ' || param[i] == '\t' {
			if start < i {
				params = append(params, param[start:i])
			}
			start = i + 1
		}
	}
	if start < len(param) {
		params = append(params, param[start:])
	}

	if len(params) < 2 {
		return false
	}

	targetField := params[0]
	validValues := params[1:]
	fieldValue := fl.Field().String()

	targetFieldValue := fl.Parent().FieldByName(targetField)
	if !targetFieldValue.IsValid() {
		return true
	}

	switch targetFieldValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		targetInt := targetFieldValue.Int()
		for _, val := range validValues {
			if intVal, err := strconv.ParseInt(val, 10, 64); err == nil && targetInt == intVal {
				return fieldValue != ""
			}
		}
	case reflect.String:
		targetStr := targetFieldValue.String()
		for _, val := range validValues {
			if targetStr == val {
				return fieldValue != ""
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		targetUint := targetFieldValue.Uint()
		for _, val := range validValues {
			if uintVal, err := strconv.ParseUint(val, 10, 64); err == nil && targetUint == uintVal {
				return fieldValue != ""
			}
		}
	case reflect.Float32, reflect.Float64:
		targetFloat := targetFieldValue.Float()
		for _, val := range validValues {
			if floatVal, err := strconv.ParseFloat(val, 64); err == nil && targetFloat == floatVal {
				return fieldValue != ""
			}
		}
	default:
		return false
	}

	return true
}

type translation struct {
	tag             string
	translation     string
	override        bool
	customRegisFunc validator.RegisterTranslationsFunc
	customTransFunc validator.TranslationFunc
}

// customRegisTranslation 自定义校验错误信息
func customRegisTranslation() error {
	translations := []translation{
		{tag: "phone_number", translation: "{0}格式不正确", override: false},
		{tag: "required_if_exist", translation: "{0}字段必填", override: false},
		{tag: "regexp", translation: "{0}字段规则不匹配", override: false},
	}

	return registerTranslation(translations)
}

// registerTranslation 注册翻译
func registerTranslation(translations []translation) error {
	for _, t := range translations {
		var regFunc validator.RegisterTranslationsFunc
		if t.customRegisFunc != nil {
			regFunc = t.customRegisFunc
		} else {
			regFunc = registrationFunc(t.tag, t.translation, t.override)
		}

		var transFunc validator.TranslationFunc
		if t.customTransFunc != nil {
			transFunc = t.customTransFunc
		} else {
			transFunc = translateFunc
		}

		err := validate.RegisterTranslation(t.tag, trans, regFunc, transFunc)
		if err != nil {
			return err
		}
	}
	return nil
}

// registrationFunc 校验规则注册函数
func registrationFunc(tag string, translation string, override bool) validator.RegisterTranslationsFunc {
	return func(ut ut.Translator) (err error) {
		if err = ut.Add(tag, translation, override); err != nil {
			return
		}
		return
	}
}

// translateFunc 校验规则翻译函数
func translateFunc(ut ut.Translator, fe validator.FieldError) string {
	t, err := ut.T(fe.Tag(), fe.Field())
	if err != nil {
		log.Logger.Warn("警告: 翻译字段错误", zap.Any("Error reason", fe))
		return fe.Error()
	}

	return t
}
