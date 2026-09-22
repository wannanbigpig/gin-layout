package admin_v1

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
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
		if handleErr := loginService.HandleLoginFailure(params.UserName, loginService.ExtractErrorMessage(err), logInfo, false); handleErr != nil {
			// 登录本身已经失败；失败处理链路的写库错误只记录，不覆盖原始业务提示。
			loginService.LogLoginFailureHandleError(params.UserName, handleErr)
		}
		api.Err(c, err)
		return
	}

	// 校验验证码
	if !captcha.Verify(params.CaptchaID, params.Captcha) {
		// 记录验证码错误日志
		if handleErr := loginService.HandleLoginFailure(params.UserName, "验证码错误", logInfo, true); handleErr != nil {
			loginService.LogLoginFailureHandleError(params.UserName, handleErr)
		}
		api.FailCode(c, errors.CaptchaErr)
		return
	}

	// 执行登录
	result, err := loginService.Login(params.UserName, params.PassWord, logInfo)
	if err != nil {
		api.Err(c, err)
		return
	}
	loginService.SetRefreshCookie(c, result.RefreshToken)
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

// LoginOptions 返回登录页可用的认证方式，不暴露供应商或凭据信息。
func (api LoginController) LoginOptions(c *gin.Context) {
	api.Success(c, auth.NewSMSLoginService().Options())
}

// SendSMSCode 发送短信登录验证码。响应不区分手机号是否存在，避免账号枚举。
func (api LoginController) SendSMSCode(c *gin.Context) {
	params := form.NewSMSCodePayload()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if !captcha.Verify(params.CaptchaID, params.Captcha) {
		api.FailCode(c, errors.CaptchaErr)
		return
	}
	result, err := auth.NewSMSLoginService().SendCode(c.Request.Context(), params.CountryCode, params.PhoneNumber, c.ClientIP())
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// SMSLogin 使用一次性短信验证码登录。
func (api LoginController) SMSLogin(c *gin.Context) {
	params := form.NewSMSLoginPayload()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	loginService := auth.NewLoginService()
	result, err := auth.NewSMSLoginService().Login(c.Request.Context(), params.CountryCode, params.PhoneNumber, params.ChallengeID, params.SMSCode, loginService.BuildLoginLogInfo(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	loginService.SetRefreshCookie(c, result.RefreshToken)
	api.Success(c, result)
}

// Logout 管理员用户退出登录
func (api LoginController) Logout(c *gin.Context) {
	loginService := auth.NewLoginService()
	accessToken, err := req.GetAccessToken(c)
	if err == nil {
		if err := loginService.Logout(accessToken); err != nil {
			log.Logger.Warn("退出登录时撤销 access token 失败，继续清理 refresh cookie", zap.Error(err))
		}
	}
	if refreshToken, cookieErr := loginService.RefreshTokenFromCookie(c); cookieErr == nil {
		if err := loginService.RevokeRefreshToken(refreshToken, model.RevokedCodeUserLogout, "用户主动登出（退出登录）"); err != nil {
			log.Logger.Warn("退出登录时撤销 refresh token 失败，继续清理 refresh cookie", zap.Error(err))
		}
	}
	loginService.ClearRefreshCookie(c)
	diff := auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{
		"action": "logout",
	}, []auditdiff.FieldRule{
		{Field: "action", Label: "操作"},
	}))
	middleware.SetAuditChangeDiffRaw(c, diff)
	api.Success(c, nil)
}

func (api LoginController) Refresh(c *gin.Context) {
	loginService := auth.NewLoginService()
	refreshToken, err := loginService.RefreshTokenFromCookie(c)
	if err != nil {
		api.FailCode(c, errors.RefreshTokenInvalid)
		return
	}
	logInfo := loginService.BuildLoginLogInfo(c)
	result, newRefreshToken, err := loginService.RefreshByToken(refreshToken, logInfo)
	if err != nil {
		api.Err(c, err)
		return
	}
	loginService.SetRefreshCookie(c, newRefreshToken)
	api.Success(c, result)
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
