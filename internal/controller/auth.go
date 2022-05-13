package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/pkg/error_code"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func Login(c *gin.Context) {
	loginForm := form.LoginForm()
	if err := validator.CheckPostParams(c, &loginForm); err != nil {
		return
	}
	result, err := service.Login(loginForm.UserName, loginForm.PassWord)
	if err != nil {
		resp().FailCode(c, 1, err.Error())
		return
	}

	resp().WithData(result).Success(c)
}

func Register(c *gin.Context) {
	resp().FailCode(c, error_code.RBACError)
}
