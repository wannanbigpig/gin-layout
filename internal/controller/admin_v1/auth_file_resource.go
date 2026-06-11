package admin_v1

import (
	stderrors "errors"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	pkgErrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// FileResourceController 文件资源管理控制器。
type FileResourceController struct {
	controller.Api
}

// FolderStats 查询目录统计信息（文件数和子目录数）。
func (api FileResourceController) FolderStats(c *gin.Context) {
	query := form.NewFileFolderStatsForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}
	result, err := service.NewFileResourceService().FolderStats(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
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

// Export 异步导出文件资源 CSV，提交到队列后返回任务信息。
func (api FileResourceController) Export(c *gin.Context) {
	params := form.NewFileResourceListQuery()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	payload := map[string]any{
		"origin_name":       params.OriginName,
		"uuid":              params.UUID,
		"mime_type":         params.MimeType,
		"file_type":         params.FileType,
		"is_public":         params.IsPublic,
		"folder_id":         params.FolderID,
		"include_subfolder": params.IncludeSubfolder,
		"storage_driver":    params.StorageDriver,
		"storage_status":    params.StorageStatus,
		"is_referenced":     params.IsReferenced,
		"is_deleted":        params.IsDeleted,
		"uid":               params.UID,
		"start_time":        params.StartTime,
		"end_time":          params.EndTime,
		"limit":             params.Limit,
	}

	submitAsyncExport(api.Api, c, asyncExportRequest{
		taskCode:  jobs.TypeExportFileList,
		queueName: jobs.ExportFileListQueue,
		payload:   payload,
	})
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

func (api FileResourceController) BatchDelete(c *gin.Context) {
	params := form.NewFileBatchDeleteForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	result, err := service.NewFileResourceService().BatchDelete(params.IDs, api.GetCurrentUserID(c), params.DeletedReason)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
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

func (api FileResourceController) BatchRestore(c *gin.Context) {
	params := form.NewFileBatchRestoreForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := service.NewFileResourceService().BatchRestore(params.IDs); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api FileResourceController) BatchDestroy(c *gin.Context) {
	params := form.NewFileBatchDestroyForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().BatchDestroy(params.IDs)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
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
	params := form.NewFileFolderTreeForm()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	includeHidden := params.IncludeHidden != nil && *params.IncludeHidden == global.Yes
	result, err := service.NewFileResourceService().FolderTree(includeHidden)
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
	if err := service.NewFileResourceService().DeleteFolder(params.ID, api.GetCurrentUserID(c), params.DeletedReason); err != nil {
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
	files := multipartForm.File["files"]
	result, err := service.NewFileResourceService().UploadLocal(files, params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	if len(files) <= 1 {
		if result != nil && len(result.Items) > 0 {
			api.Success(c, result.Items[0])
			return
		}
		api.Success(c, nil)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) UploadCredential(c *gin.Context) {
	params := form.NewFileUploadCredentialForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().UploadCredential(params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) UploadCredentialBatch(c *gin.Context) {
	params := form.NewFileUploadCredentialBatchForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().UploadCredentialBatch(params, api.GetCurrentUserID(c))
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
	result, err := service.NewFileResourceService().CompleteDirectUploadWithContext(c.Request.Context(), params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api FileResourceController) UploadCompleteBatch(c *gin.Context) {
	params := form.NewFileUploadCompleteBatchForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().CompleteDirectUploadBatchWithContext(c.Request.Context(), params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// MultipartInit 分片上传初始化。
func (api FileResourceController) MultipartInit(c *gin.Context) {
	params := form.NewFileMultipartInitForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().MultipartInit(params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// MultipartComplete 分片上传完成。
func (api FileResourceController) MultipartComplete(c *gin.Context) {
	params := form.NewFileMultipartCompleteForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := service.NewFileResourceService().MultipartCompleteWithContext(c.Request.Context(), params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// MultipartAbort 分片上传中止。
func (api FileResourceController) MultipartAbort(c *gin.Context) {
	params := form.NewFileMultipartAbortForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := service.NewFileResourceService().MultipartAbort(params); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}
