package func_make

import (
	"testing"
)

var (
	funcMap = map[string]interface{}{
		"test": func(str string) string {
			return str
		},
	}
	funcMake = New()
)

func TestRegisters(t *testing.T) {
	err := funcMake.Registers(funcMap)
	if err != nil {
		t.Errorf("绑定失败")
	}
}

func TestRegister(t *testing.T) {
	err := funcMake.Register("test1", func(str ...string) string {
		var res string
		for _, v := range str {
			res += v
		}
		return res
	})
	if err != nil {
		t.Errorf("绑定失败")
	}
}

func TestCall(t *testing.T) {
	TestRegisters(t)
	TestRegister(t)
	if _, err := funcMake.Call("test", "1"); err != nil {
		t.Errorf("请求test方法失败:%s", err)
	}
	if _, err := funcMake.Call("test1", "2323", "ddd"); err != nil {
		t.Errorf("请求test1方法失败:%s", err)
	}
}
