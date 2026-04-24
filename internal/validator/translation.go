package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"go.uber.org/zap"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

func initTranslators(validate *validator.Validate) (map[string]ut.Translator, error) {
	zhT := zh.New()
	enT := en.New()
	uni := ut.New(enT, zhT, enT)

	locales := []string{"zh", "en"}
	translators := make(map[string]ut.Translator, len(locales))
	for _, locale := range locales {
		trans, ok := uni.GetTranslator(locale)
		if !ok {
			return nil, fmt.Errorf("validator translator locale not supported: %s", locale)
		}
		if err := registerLocaleTranslations(validate, trans, locale); err != nil {
			return nil, err
		}
		translators[locale] = trans
	}
	return translators, nil
}

func registerLocaleTranslations(validate *validator.Validate, trans ut.Translator, locale string) error {
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
		return fmt.Errorf("注册默认翻译器失败: %w", err)
	}

	if err := customRegisTranslation(validate, trans, locale); err != nil {
		return fmt.Errorf("注册自定义翻译失败: %w", err)
	}
	return nil
}

type translation struct {
	tag             string
	translation     string
	override        bool
	customRegisFunc validator.RegisterTranslationsFunc
	customTransFunc validator.TranslationFunc
}

func customRegisTranslation(validate *validator.Validate, trans ut.Translator, locale string) error {
	return registerTranslation(validate, trans, localeTranslations(locale))
}

func localeTranslations(locale string) []translation {
	switch normalizeValidatorLocale(locale) {
	case "en":
		return []translation{
			{tag: "phone_number", translation: "{0} format is invalid", override: false},
			{tag: "required_if_exist", translation: "{0} is required", override: false},
			{tag: "regexp", translation: "{0} format is invalid", override: false},
		}
	default:
		return []translation{
			{tag: "phone_number", translation: "{0}格式不正确", override: false},
			{tag: "required_if_exist", translation: "{0}字段必填", override: false},
			{tag: "regexp", translation: "{0}字段规则不匹配", override: false},
		}
	}
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

func normalizeValidatorLocale(locale string) string {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	switch {
	case strings.HasPrefix(normalized, "en"):
		return "en"
	default:
		return "zh"
	}
}
