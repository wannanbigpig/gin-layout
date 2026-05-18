package admin_v1

import (
	stderrors "errors"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	pkgErrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// FileResourceController 文件资源管理控制器。
type FileResourceController struct {
	controller.Api
}

func NewFileResourceController() *FileResourceController {
	return &FileResourceController{}
}

// List 分页查询文件资源列表。
func (api FileResourceController) List(c *gin.Context) {
	params := form.NewFileResourceListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := service.NewFileResourceService().List(params)
	api.Success(c, result)
}

// Detail 查询文件资源详情。
func (api FileResourceController) Detail(c *gin.Context) {
	query := form.NewFileResourceIDForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := service.NewFileResourceService().Detail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, detail)
}

// Delete 删除文件资源。
func (api FileResourceController) Delete(c *gin.Context) {
	params := form.NewFileResourceIDForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := service.NewFileResourceService().Delete(params.ID, api.GetCurrentUserID(c), params.DeletedReason); err != nil {
		var referencedErr *service.FileReferencedDeleteError
		if stderrors.As(err, &referencedErr) {
			message := "文件存在引用，不能删除"
			businessErr := referencedErr.BusinessError()
			if businessErr != nil {
				message = businessErr.GetMessage()
			}
			api.Fail(c, pkgErrors.FileReferenced, message, gin.H{"references": referencedErr.References})
			return
		}
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api FileResourceController) TrashList(c *gin.Context) {
	params := form.NewFileResourceListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	deleted := uint8(1)
	params.IsDeleted = &deleted
	result := service.NewFileResourceService().List(params)
	api.Success(c, result)
}

func (api FileResourceController) Restore(c *gin.Context) {
	params := form.NewFileResourceIDForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := service.NewFileResourceService().Restore(params.ID); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api FileResourceController) Destroy(c *gin.Context) {
	params := form.NewFileResourceIDForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := service.NewFileResourceService().Destroy(params.ID); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api FileResourceController) References(c *gin.Context) {
	params := form.NewFileReferenceListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	result := service.NewFileResourceService().References(params)
	api.Success(c, result)
}

func (api FileResourceController) FolderTree(c *gin.Context) {
	result, err := service.NewFileResourceService().FolderTree()
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) FolderCreate(c *gin.Context) {
	params := form.NewFileFolderCreateForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().CreateFolder(params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) FolderUpdate(c *gin.Context) {
	params := form.NewFileFolderUpdateForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().UpdateFolder(params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) FolderDelete(c *gin.Context) {
	params := form.NewFileFolderDeleteForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := service.NewFileResourceService().DeleteFolder(params.ID); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api FileResourceController) FolderMove(c *gin.Context) {
	params := form.NewFileFolderMoveForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().MoveFolder(params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) Move(c *gin.Context) {
	params := form.NewFileMoveForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().MoveFiles(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) UploadLocal(c *gin.Context) {
	params := form.NewFileLocalUploadForm()
	if err := c.ShouldBind(params); err != nil {
		api.FailCode(c, pkgErrors.InvalidParameter)
		return
	}
	multipartForm, err := c.MultipartForm()
	if err != nil {
		api.FailCode(c, pkgErrors.InvalidParameter)
		return
	}
	result, err := service.NewFileResourceService().UploadLocal(multipartForm.File["files"], params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) UploadCredential(c *gin.Context) {
	params := form.NewFileUploadCredentialForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().UploadCredential(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) UploadComplete(c *gin.Context) {
	params := form.NewFileUploadCompleteForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().CompleteDirectUpload(params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}
