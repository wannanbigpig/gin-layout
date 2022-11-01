package command

import (
	"fmt"
	"github.com/wannanbigpig/gin-layout/internal/pkg/func_make"
)

var (
	commandMap = map[string]interface{}{
		"demo": demo,
	}
	funcMake = func_make.New()
)

func Register() {
	err := funcMake.Registers(commandMap)
	if err != nil {
		panic("failed to register console command: " + err.Error())
	}
}

func Run(funcName string) {
	Register()
	_, err := funcMake.Call(funcName)
	if err != nil {
		fmt.Printf("execution failed, error cause: %v \n", err.Error())
		return
	}
	fmt.Printf("complete! \n")
}
