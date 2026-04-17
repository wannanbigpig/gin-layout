package validator

import (
	"errors"
	"reflect"
	"sync"

	"github.com/gin-gonic/gin/binding"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

var validatorRuntime = newValidatorRuntime()

type validatorRuntimeState struct {
	once             sync.Once
	validate         *validator.Validate
	trans            ut.Translator
	translatorLocale string
	rulesReady       bool
	tagNameReady     bool
	initErr          error
}

func newValidatorRuntime() *validatorRuntimeState {
	return &validatorRuntimeState{}
}

// InitValidatorTrans 初始化验证器和翻译器。
func InitValidatorTrans(locale string) error {
	err := validatorRuntime.initOnce(locale)
	if err != nil && log.Logger != nil {
		log.Logger.Error("初始化 validator 失败", zap.String("locale", locale), zap.Error(err))
	}
	return err
}

func (s *validatorRuntimeState) initOnce(locale string) error {
	s.once.Do(func() {
		s.initErr = s.init(locale)
	})
	return s.initErr
}

func (s *validatorRuntimeState) init(locale string) error {
	engine, ok := getValidatorEngine()
	if !ok {
		return errors.New("初始化 validator 失败")
	}
	s.validate = engine

	if err := s.ensureRules(); err != nil {
		return err
	}
	s.ensureTagNameFunc()

	trans, err := initTranslator(s.validate, locale)
	if err != nil {
		return err
	}
	s.trans = trans
	s.translatorLocale = locale
	return nil
}

func (s *validatorRuntimeState) ensureRules() error {
	if s.rulesReady {
		return nil
	}
	if err := initCustomRules(s.validate); err != nil {
		return err
	}
	s.rulesReady = true
	return nil
}

func (s *validatorRuntimeState) ensureTagNameFunc() {
	if s.tagNameReady {
		return
	}
	registerTagNameFunc(s.validate)
	s.tagNameReady = true
}

func getValidatorEngine() (*validator.Validate, bool) {
	engine := binding.Validator.Engine()
	if engine == nil {
		return nil, false
	}
	validate, ok := engine.(*validator.Validate)
	return validate, ok
}

func registerTagNameFunc(validate *validator.Validate) {
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
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
