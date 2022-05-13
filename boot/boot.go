package boot

import (
	_ "l-admin.com/data"
	"l-admin.com/internal/validator"
	"l-admin.com/pkg/logger"
)

func init() {
	// 1、初始化zap日志
	logger.InitLogger()

	// 2、初始化验证器
	if err := validator.InitValidatorTrans("zh"); err != nil {
		panic("验证器初始化失败")
	}
}
