package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	req "github.com/wannanbigpig/gin-layout/internal/pkg/request"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"github.com/wannanbigpig/gin-layout/pkg/utils/captcha"
)

// LoginController 登录控制器
type LoginController struct {
	controller.Api
}

// NewLoginController 创建登录控制器实例
func NewLoginController() *LoginController {
	return &LoginController{}
}

// Login 管理员用户登录
func (api LoginController) Login(c *gin.Context) {
	params := form.NewLoginForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	// 构建登录日志信息
	loginService := auth.NewLoginService()
	logInfo := loginService.BuildLoginLogInfo(c)
	if err := loginService.CheckLoginAllowed(params.UserName); err != nil {
		loginService.HandleLoginFailure(params.UserName, loginService.ExtractErrorMessage(err), logInfo, false)
		api.Err(c, err)
		return
	}

	// 校验验证码
	if !captcha.Verify(params.CaptchaID, params.Captcha) {
		// 记录验证码错误日志
		loginService.HandleLoginFailure(params.UserName, "验证码错误", logInfo, true)
		api.FailCode(c, errors.CaptchaErr)
		return
	}

	// 执行登录
	result, err := loginService.Login(params.UserName, params.PassWord, logInfo)
	if err != nil {
		api.Err(c, err)
		return
	}
	diff := auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{
		"action":   "login",
		"username": params.UserName,
	}, []auditdiff.FieldRule{
		{Field: "action", Label: "操作"},
		{Field: "username", Label: "用户名"},
	}))
	middleware.SetAuditChangeDiffRaw(c, diff)
	api.Success(c, result)
}

// LoginCaptcha 生成登录验证码
func (api LoginController) LoginCaptcha(c *gin.Context) {
	result, err := captcha.Generate()
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}

// Logout 管理员用户退出登录
func (api LoginController) Logout(c *gin.Context) {
	accessToken, err := req.GetAccessToken(c)
	if err != nil {
		// Token提取失败，视为已退出
		api.Success(c, nil)
		return
	}

	if err := auth.NewLoginService().Logout(accessToken); err != nil {
		api.Err(c, err)
		return
	}
	diff := auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{
		"action": "logout",
	}, []auditdiff.FieldRule{
		{Field: "action", Label: "操作"},
	}))
	middleware.SetAuditChangeDiffRaw(c, diff)
	api.Success(c, nil)
}

// CheckToken 检查Token是否有效
func (api LoginController) CheckToken(c *gin.Context) {
	accessToken, err := req.GetAccessToken(c)
	if err != nil {
		api.Err(c, err)
		return
	}

	loginService := auth.NewLoginService()
	loginService.SetCtx(c)
	_, ok := loginService.CheckToken(accessToken)

	api.Success(c, ok)
}
