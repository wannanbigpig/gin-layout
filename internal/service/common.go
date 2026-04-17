package service

import (
	"errors"
	"mime/multipart"
	"path/filepath"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
	"go.uber.org/zap"
)

// CommonService 通用服务
type CommonService struct {
	Base
}

const maxUploadFileSize int64 = 10 * 1024 * 1024
const partialImageUploadFailed = "部分图片上传失败"

// NewCommonService 创建通用服务实例。
func NewCommonService() *CommonService {
	return &CommonService{}
}

// UploadImages 批量上传图片。
func (s CommonService) UploadImages(files []*multipart.FileHeader, path string) ([]*utils.FileInfo, error) {
	filesInfo := make([]*utils.FileInfo, 0, len(files))
	for _, fileHeader := range files {
		file, err := s.UploadImage(fileHeader, true, path)
		if err != nil {
			log.Logger.Warn("文件上传失败",
				zap.String("filename", fileHeader.Filename),
				zap.Error(err),
			)
		}
		filesInfo = append(filesInfo, file)
	}

	return summarizeImageUploadResults(filesInfo)
}

// UploadImage 上传单张图片并保存文件记录。
func (s CommonService) UploadImage(fileHeader *multipart.FileHeader, isPublic bool, path string) (*utils.FileInfo, error) {
	fileInfo := initUploadResult(fileHeader)
	isPublicFlag := visibilityFlag(isPublic)
	basePath := storageBasePath(isPublic)
	uploadPath, err := normalizeUploadPath(path)
	if err != nil {
		return setFileFailure(fileInfo, "上传目录不合法", err)
	}
	uploadDir, err := resolveUploadDestination(basePath, uploadPath)
	if err != nil {
		return setFileFailure(fileInfo, "上传目录不合法", err)
	}

	if fileHeader.Size > maxUploadFileSize {
		return setFileFailure(fileInfo, "文件大小不能大于10M", nil)
	}

	result, err := utils.SaveUploadedImageWithUUID(fileHeader, uploadDir)
	if err != nil {
		if errors.Is(err, utils.ErrInvalidImageType) {
			return setFileFailure(fileInfo, "仅支持图片格式", err)
		}
		return setFileFailure(fileInfo, "文件保存失败", err)
	}
	savedPath := result.Path
	fileInfo.Sha256 = result.Sha256

	existingFile, err := findReusableUploadFile(result.Sha256, isPublicFlag)
	if err == nil && existingUploadFileExists(basePath, existingFile.Path) {
		cleanupStoredUpload(result.Path)
		fillFileInfoFromModel(fileInfo, existingFile)
		return fileInfo, nil
	}

	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		cleanupStoredUpload(result.Path)
		return setFileFailure(fileInfo, "上传路径获取异常", err)
	}
	relPath, err := filepath.Rel(absBasePath, result.Path)
	if err != nil {
		cleanupStoredUpload(result.Path)
		return setFileFailure(fileInfo, "上传路径获取异常", err)
	}
	result.Path = relPath

	uploadFileModel := model.NewUploadFiles()
	uploadFileModel.UID = s.GetAdminUserId()
	uploadFileModel.OriginName = result.OriginName
	uploadFileModel.Name = result.Name
	uploadFileModel.Path = result.Path
	uploadFileModel.Size = uint(result.Size)
	uploadFileModel.Ext = result.Ext
	uploadFileModel.Hash = result.Sha256
	uploadFileModel.UUID = result.UUID
	uploadFileModel.MimeType = result.MimeType
	uploadFileModel.IsPublic = isPublicFlag

	if err := uploadFileModel.Create(); err != nil {
		cleanupStoredUpload(savedPath)
		return setFileFailure(fileInfo, "保存文件信息失败", err)
	}

	fillFileInfoFromUploadResult(fileInfo, result)
	return fileInfo, nil
}

// GetFileAccessPath 获取文件访问路径
// fileUUID: 文件UUID（32位十六进制字符串，不带连字符），用于URL访问
// checkAuth: 是否检查权限（私有文件需要检查）
// currentUID: 当前用户ID（用于权限检查，0表示未登录）
func (s CommonService) GetFileAccessPath(fileUUID string, checkAuth bool, currentUID uint) (string, error) {
	if len(fileUUID) != 32 {
		return "", e.NewBusinessError(e.FileIdentifierInvalid, "文件标识格式错误，应使用32位UUID")
	}

	uploadFile := model.NewUploadFiles()
	// 通过UUID查询（更短，适合URL）
	err := uploadFile.GetDetail("uuid = ?", fileUUID)
	if err != nil {
		return "", e.NewBusinessError(e.NotFound, "文件不存在")
	}

	if uploadFile.IsPublic == global.No {
		if !checkAuth || currentUID == 0 {
			return "", e.NewBusinessError(e.FilePrivateAuthNeeded, "该文件为私有文件，需要登录认证")
		}
		if uploadFile.UID != currentUID {
			return "", e.NewBusinessError(e.FileAccessDenied, "无权访问该文件")
		}
	}

	filePath, err := resolveUploadDestination(storageBasePath(uploadFile.IsPublic == global.Yes), uploadFile.Path)
	if err != nil {
		return "", e.NewBusinessError(e.FileAccessDenied, "文件路径异常，无法访问")
	}
	return filePath, nil
}
