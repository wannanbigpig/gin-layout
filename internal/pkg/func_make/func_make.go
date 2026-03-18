package func_make

import (
	"errors"
	"reflect"
)

// FuncMap 保存可按名称调用的函数映射。
type FuncMap map[string]reflect.Value

// New 创建一个空的函数映射表。
func New() FuncMap {
	return make(FuncMap, 2)
}

// Register 注册单个函数。
func (f FuncMap) Register(name string, fn any) error {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return errors.New(name + " is not a function type.")
	}
	f[name] = v
	return nil
}

// Registers 批量注册函数。
func (f FuncMap) Registers(funcMap map[string]any) (err error) {
	for k, v := range funcMap {
		err = f.Register(k, v)
		if err != nil {
			break
		}
	}
	return
}

// Call 按名称调用已注册函数。
func (f FuncMap) Call(name string, params ...any) (result []reflect.Value, err error) {
	if _, ok := f[name]; !ok {
		err = errors.New(name + " method does not exist.")
		return
	}
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		in[k] = reflect.ValueOf(param)
	}

	defer func() {
		if e := recover(); e != nil {
			err = errors.New("call " + name + " method fail. " + e.(string))
		}
	}()

	result = f[name].Call(in)
	return
}
