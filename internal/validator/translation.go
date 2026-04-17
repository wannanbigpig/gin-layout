package validator

import (
	"fmt"

	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"go.uber.org/zap"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

func initTranslator(validate *validator.Validate, locale string) (ut.Translator, error) {
	zhT := zh.New()
	enT := en.New()
	uni := ut.New(enT, zhT, enT)

	trans, ok := uni.GetTranslator(locale)
	if !ok {
		return nil, fmt.Errorf("validator 不支持语言 %s", locale)
	}

	var err error
	switch locale {
	case "en":
		err = enTranslations.RegisterDefaultTranslations(validate, trans)
	case "zh":
		err = zhTranslations.RegisterDefaultTranslations(validate, trans)
	default:
		err = enTranslations.RegisterDefaultTranslations(validate, trans)
	}
	if err != nil {
		return nil, fmt.Errorf("注册默认翻译器失败: %w", err)
	}

	if err := customRegisTranslation(validate, trans); err != nil {
		return nil, fmt.Errorf("注册自定义翻译失败: %w", err)
	}
	return trans, nil
}

type translation struct {
	tag             string
	translation     string
	override        bool
	customRegisFunc validator.RegisterTranslationsFunc
	customTransFunc validator.TranslationFunc
}

func customRegisTranslation(validate *validator.Validate, trans ut.Translator) error {
	translations := []translation{
		{tag: "phone_number", translation: "{0}格式不正确", override: false},
		{tag: "required_if_exist", translation: "{0}字段必填", override: false},
		{tag: "regexp", translation: "{0}字段规则不匹配", override: false},
	}

	return registerTranslation(validate, trans, translations)
}

func registerTranslation(validate *validator.Validate, trans ut.Translator, translations []translation) error {
	for _, t := range translations {
		regFunc := t.customRegisFunc
		if regFunc == nil {
			regFunc = registrationFunc(t.tag, t.translation, t.override)
		}

		transFunc := t.customTransFunc
		if transFunc == nil {
			transFunc = translateFunc
		}

		if err := validate.RegisterTranslation(t.tag, trans, regFunc, transFunc); err != nil {
			return err
		}
	}
	return nil
}

func registrationFunc(tag string, translation string, override bool) validator.RegisterTranslationsFunc {
	return func(ut ut.Translator) (err error) {
		if err = ut.Add(tag, translation, override); err != nil {
			return
		}
		return
	}
}

func translateFunc(ut ut.Translator, fe validator.FieldError) string {
	t, err := ut.T(fe.Tag(), fe.Field())
	if err != nil {
		log.Logger.Warn("警告: 翻译字段错误", zap.Any("Error reason", fe))
		return fe.Error()
	}
	return t
}
