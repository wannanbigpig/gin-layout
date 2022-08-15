package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type AuthController struct {
	controller.Api
}

func NewAuthController() *AuthController {
	return &AuthController{}
}

// Login 一个跑通全流程的示例，业务代码未补充完整
func (api *AuthController) Login(c *gin.Context) {
	// 初始化参数结构体
	loginForm := form.LoginForm()
	// 绑定参数并使用验证器验证参数
	if err := validator.CheckPostParams(c, &loginForm); err != nil {
		return
	}
	// 实际业务调用
	result, err := service.NewAuthService().Login(loginForm.UserName, loginForm.PassWord)
	// 根据业务返回值判断业务成功 OR 失败
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}
