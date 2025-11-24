package admin_v1

import (
	"image/color"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"github.com/mssola/useragent"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"github.com/wannanbigpig/gin-layout/pkg/utils/captcha"
)

var (
	// captchaFonts 验证码字体列表
	captchaFonts = []string{
		"wqy-microhei.ttc",
		"3Dumb.ttf",
		"actionj.ttf",
		"ApothecaryFont.ttf",
		"chromohv.ttf",
		"Comismsh.ttf",
		"DENNEthree-dee.ttf",
		"Flim-Flam.ttf",
		"RitaSmith.ttf",
	}
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
	logInfo := buildLoginLogInfo(c)

	// 校验验证码
	if !captcha.Verify(params.CaptchaID, params.Captcha) {
		// 记录验证码错误日志
		loginService := permission.NewLoginService()
		loginService.RecordLoginFailLog(params.UserName, "验证码错误", logInfo)
		api.FailCode(c, errors.CaptchaErr)
		return
	}

	// 执行登录
	result, err := permission.NewLoginService().Login(params.UserName, params.PassWord, logInfo)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}

// buildLoginLogInfo 构建登录日志信息
func buildLoginLogInfo(c *gin.Context) permission.LoginLogInfo {
	userAgentStr := c.Request.UserAgent()

	// 解析 user_agent 获取 OS 和 Browser 信息
	ua := useragent.New(userAgentStr)
	os := ua.OS()
	browser, _ := ua.Browser()

	return permission.LoginLogInfo{
		IP:        c.ClientIP(),
		UserAgent: userAgentStr,
		OS:        os,
		Browser:   browser,
	}
}

// LoginCaptcha 生成登录验证码
func (api LoginController) LoginCaptcha(c *gin.Context) {
	driver := createCaptchaDriver()
	result, err := captcha.Generate(driver, nil)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}

// createCaptchaDriver 创建验证码驱动
func createCaptchaDriver() *base64Captcha.DriverString {
	return base64Captcha.NewDriverString(
		48, 120, 6, 2, 4,
		base64Captcha.TxtAlphabet+base64Captcha.TxtNumbers,
		&color.RGBA{R: 255, G: 255, B: 255, A: 0},
		base64Captcha.DefaultEmbeddedFonts,
		captchaFonts,
	)
}

// Logout 管理员用户退出登录
func (api LoginController) Logout(c *gin.Context) {
	accessToken, err := extractAccessToken(c)
	if err != nil {
		// Token提取失败，视为已退出
		api.Success(c, nil)
		return
	}

	if err := permission.NewLoginService().Logout(accessToken); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// CheckToken 检查Token是否有效
func (api LoginController) CheckToken(c *gin.Context) {
	accessToken, err := extractAccessToken(c)
	if err != nil {
		api.Err(c, err)
		return
	}

	loginService := permission.NewLoginService()
	loginService.SetCtx(c)
	_, ok := loginService.CheckToken(accessToken)

	api.Success(c, ok)
}

// extractAccessToken 从请求头中提取访问令牌
func extractAccessToken(c *gin.Context) (string, error) {
	authorization := c.GetHeader("Authorization")
	return token.GetAccessToken(authorization)
}
