package validator

import (
	"errors"
	"reflect"
	"regexp"
	"strconv"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	phoneNumberRegex = regexp.MustCompile(`^1[3456789]\d{9}$`)
	regexCache       sync.Map // map[string]*regexp.Regexp
)

// RegexpValidator 通用正则表达式验证器。
func RegexpValidator(fl validator.FieldLevel) bool {
	param := fl.Param()
	if param == "" {
		return false
	}

	value := fl.Field().String()
	if cached, ok := regexCache.Load(param); ok {
		return cached.(*regexp.Regexp).MatchString(value)
	}

	reg, err := regexp.Compile(param)
	if err != nil {
		return false
	}
	regexCache.Store(param, reg)
	return reg.MatchString(value)
}

func initCustomRules(validate *validator.Validate) error {
	err := validate.RegisterValidation("phone_number", func(fl validator.FieldLevel) bool {
		return phoneNumberRegex.MatchString(fl.Field().String())
	})
	if err != nil {
		return errors.New("注册 phone_number 校验规则失败")
	}

	err = validate.RegisterValidation("required_if_exist", requiredIf)
	if err != nil {
		return errors.New("注册 required_if_exist 校验规则失败")
	}

	err = validate.RegisterValidation("regexp", RegexpValidator)
	if err != nil {
		return errors.New("注册 regexp 校验规则失败")
	}
	return nil
}

// requiredIf 字段B存在时，字段A必填。
func requiredIf(fl validator.FieldLevel) bool {
	param := fl.Param()
	if param == "" {
		return false
	}

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
