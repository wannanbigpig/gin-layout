package admin_v1

import (
	"mime/multipart"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service"
)

const (
	defaultUploadPath = "default"
)

// CommonController 通用控制器
type CommonController struct {
	controller.Api
}

// NewCommonController 创建通用控制器实例
func NewCommonController() *CommonController {
	return &CommonController{}
}

// Upload 上传文件
func (api CommonController) Upload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		api.FailCode(c, errors.InvalidParameter, "参数错误")
		return
	}

	// 获取用户ID
	uid := c.GetUint(global.ContextKeyUID)
	commonService := service.NewCommonService()
	commonService.SetAdminUserId(uid)

	// 获取上传路径参数
	path := getUploadPath(form)

	// 执行文件上传
	result, err := commonService.UploadImages(form.File["files"], path)
	if err != nil {
		if service.IsPartialImageUploadError(err) {
			api.Fail(c, errors.FileUploadPartialFail, err.Error(), result)
			return
		}
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}

// GetFile 获取文件（支持公开和私有文件访问）
// 公开文件：无需认证，直接访问
// 私有文件：需要认证，只能由文件所有者访问
// 路由: GET /admin/v1/file/:uuid
// 参数: uuid - 文件的UUID（32位十六进制字符串，不带连字符）
func (api CommonController) GetFile(c *gin.Context) {
	fileUUID := c.Param("uuid")
	if fileUUID == "" {
		api.FailCode(c, errors.InvalidParameter, "文件UUID不能为空")
		return
	}

	commonService := service.NewCommonService()

	// 获取当前用户ID（如果已登录）
	var currentUID uint
	var checkAuth bool
	if uid, exists := c.Get(global.ContextKeyUID); exists {
		currentUID = uid.(uint)
		checkAuth = true
		commonService.SetAdminUserId(currentUID)
	} else {
		checkAuth = false
	}

	// 获取文件路径（会自动检查权限）
	filePath, err := commonService.GetFileAccessPath(fileUUID, checkAuth, currentUID)
	if err != nil {
		api.Err(c, err)
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		api.FailCode(c, errors.NotFound, "文件不存在")
		return
	}

	// 返回文件
	c.File(filePath)
}

// getUploadPath 获取上传路径
func getUploadPath(form *multipart.Form) string {
	if len(form.Value["path"]) > 0 && form.Value["path"][0] != "" {
		return form.Value["path"][0]
	}
	return defaultUploadPath
}
