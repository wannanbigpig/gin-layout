package admin_v1

import (
	"image/color"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"github.com/wannanbigpig/gin-layout/pkg/utils/captcha"
)

type LoginController struct {
	controller.Api
}

func NewLoginController() *LoginController {
	return &LoginController{}
}

// Login 管理员用户登录
func (api LoginController) Login(c *gin.Context) {
	// 初始化参数结构体
	loginForm := form.NewLoginForm()
	// 绑定参数并使用验证器验证参数
	if err := validator.CheckPostParams(c, &loginForm); err != nil {
		return
	}

	// 校验验证码
	if !captcha.Verify(loginForm.CaptchaID, loginForm.Captcha) {
		api.FailCode(c, errors.CaptchaErr)
		return
	}
	// 根据header中的X-Client-Type判断客户端类型，不存在默认为0
	// 客户端类型：1=Web, 2=iOS, 3=Android, 4=小程序
	clientTypeStr := c.GetHeader("X-Client-Type")
	// 将字符串转换为 uint8
	clientType, _ := strconv.ParseUint(clientTypeStr, 10, 8)
	deviceId := c.GetHeader("X-Device-Id")
	deviceName := c.GetHeader("X-Device-Name")
	// 转换成uint8类型

	logInfo := permission.LoginLogInfo{
		IP:         c.ClientIP(),
		ClientType: uint8(clientType),
		DeviceId:   deviceId,
		DeviceName: deviceName,
	}
	// 实际业务调用
	result, err := permission.NewLoginService().Login(loginForm.UserName, loginForm.PassWord, logInfo)
	// 根据业务返回值判断业务成功 OR 失败
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
	return
}

func (api LoginController) LoginCaptcha(c *gin.Context) {
	driver := base64Captcha.NewDriverString(48, 120, 6, 2, 4, base64Captcha.TxtAlphabet+base64Captcha.TxtNumbers, &color.RGBA{R: 255, G: 255, B: 255, A: 0}, base64Captcha.DefaultEmbeddedFonts, []string{
		"wqy-microhei.ttc",
		"3Dumb.ttf",
		"actionj.ttf",
		"ApothecaryFont.ttf",
		"chromohv.ttf",
		"Comismsh.ttf",
		"DENNEthree-dee.ttf",
		"Flim-Flam.ttf",
		"RitaSmith.ttf",
	})
	result, err := captcha.Generate(driver, nil)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
	return
}

// Logout 管理员用户退出登录
func (api LoginController) Logout(c *gin.Context) {
	// Get the access token from the request header
	authorization := c.GetHeader("Authorization")
	accessToken, err := token.GetAccessToken(authorization)
	if err != nil {
		api.Success(c, nil)
		return
	}
	err = permission.NewLoginService().Logout(accessToken)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
	return
}
