package service

import (
	"errors"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	c "github.com/wannanbigpig/gin-layout/config"
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
	fileInfo := &utils.FileInfo{
		OriginName: fileHeader.Filename,
		Size:       fileHeader.Size,
		Ext:        filepath.Ext(fileHeader.Filename),
		Status:     global.ERROR,
	}
	isPublicFlag := visibilityFlag(isPublic)
	basePath := storageBasePath(isPublic)
	uploadPath := normalizeUploadPath(path)

	if fileHeader.Size > maxUploadFileSize {
		return setFileFailure(fileInfo, "文件大小不能大于10M", nil)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return setFileFailure(fileInfo, "文件读取失败", err)
	}
	defer file.Close()

	sha256, _, err := utils.GetFileSha256AndSizeFromHeader(file)
	if err != nil {
		return setFileFailure(fileInfo, "文件校验失败", err)
	}
	fileInfo.Sha256 = sha256

	_, allowed, err := utils.IsAllowedImage(file)
	if err != nil || !allowed {
		return setFileFailure(fileInfo, "仅支持图片格式", err)
	}

	existingFile, err := findReusableUploadFile(sha256, isPublicFlag)
	if err == nil && existingUploadFileExists(basePath, existingFile.Path) {
		fillFileInfoFromModel(fileInfo, existingFile)
		return fileInfo, nil
	}

	result, err := utils.UploadFile(fileHeader, filepath.Join(basePath, uploadPath))
	if err != nil {
		return setFileFailure(fileInfo, "文件保存失败", err)
	}

	relPath, err := filepath.Rel(basePath, result.Path)
	if err != nil {
		return setFileFailure(fileInfo, "上传路径获取异常", err)
	}
	result.Path = relPath

	uploadFileModel := model.UploadFiles{
		UID:        s.GetAdminUserId(),
		OriginName: result.OriginName,
		Name:       result.Name,
		Path:       result.Path,
		Size:       uint(result.Size),
		Ext:        result.Ext,
		Hash:       result.Sha256,
		UUID:       result.UUID,
		MimeType:   result.MimeType,
		IsPublic:   isPublicFlag,
	}
	db, err := model.GetDB(model.NewUploadFiles())
	if err == nil {
		err = db.Create(&uploadFileModel).Error
	}
	if err != nil {
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

	return filepath.Join(storageBasePath(uploadFile.IsPublic == global.Yes), uploadFile.Path), nil
}

// buildFileURL 构建文件访问完整URL
func buildFileURL(uuid string) string {
	if uuid == "" {
		return ""
	}
	// 拼接文件访问地址
	baseURL := strings.TrimSuffix(c.GetConfig().BaseURL, "/")
	if baseURL == "" {
		// 如果未配置BaseURL，返回相对路径（前端需要自己处理）
		return "/admin/v1/file/" + uuid
	}
	// 拼接完整的URL地址
	return baseURL + "/admin/v1/file/" + uuid
}

func setFileFailure(info *utils.FileInfo, reason string, err error) (*utils.FileInfo, error) {
	info.FailureReason = reason
	info.Status = global.ERROR
	return info, err
}

// visibilityFlag 返回数据库使用的公开标识。
func visibilityFlag(isPublic bool) uint8 {
	if isPublic {
		return global.Yes
	}
	return global.No
}

// storageBasePath 返回公开或私有文件的存储根目录。
func storageBasePath(isPublic bool) string {
	cfg := c.GetConfig()
	if isPublic {
		return filepath.Join(cfg.BasePath, "storage/public")
	}
	return filepath.Join(cfg.BasePath, "storage/private")
}

// normalizeUploadPath 返回有效的上传子目录。
func normalizeUploadPath(path string) string {
	if path == "" {
		return "default"
	}
	return path
}

// findReusableUploadFile 按哈希和值可见性查找可复用文件记录。
func findReusableUploadFile(hash string, isPublic uint8) (*model.UploadFiles, error) {
	uploadFile := model.NewUploadFiles()
	if err := uploadFile.GetDetail("hash = ? AND is_public = ?", hash, isPublic); err != nil {
		return nil, err
	}
	return uploadFile, nil
}

// existingUploadFileExists 判断复用文件在磁盘上是否仍然存在。
func existingUploadFileExists(basePath, relativePath string) bool {
	_, err := os.Stat(filepath.Join(basePath, relativePath))
	return err == nil
}

// fillFileInfoFromModel 将数据库文件记录复制到返回结构。
func fillFileInfoFromModel(fileInfo *utils.FileInfo, uploadFile *model.UploadFiles) {
	fileInfo.Path = uploadFile.Path
	fileInfo.Name = uploadFile.Name
	fileInfo.Size = int64(uploadFile.Size)
	fileInfo.Ext = uploadFile.Ext
	fileInfo.Sha256 = uploadFile.Hash
	fileInfo.UUID = uploadFile.UUID
	fileInfo.MimeType = uploadFile.MimeType
	fileInfo.URL = buildFileURL(uploadFile.UUID)
	fileInfo.Status = global.SUCCESS
}

// fillFileInfoFromUploadResult 将上传结果复制到返回结构。
func fillFileInfoFromUploadResult(fileInfo *utils.FileInfo, result *utils.FileInfo) {
	fileInfo.Path = result.Path
	fileInfo.Name = result.Name
	fileInfo.Size = result.Size
	fileInfo.Ext = result.Ext
	fileInfo.Sha256 = result.Sha256
	fileInfo.UUID = result.UUID
	fileInfo.MimeType = result.MimeType
	fileInfo.URL = buildFileURL(result.UUID)
	fileInfo.Status = global.SUCCESS
}

func summarizeImageUploadResults(filesInfo []*utils.FileInfo) ([]*utils.FileInfo, error) {
	if len(filesInfo) == 0 {
		return filesInfo, nil
	}

	successCount := 0
	for _, item := range filesInfo {
		if item != nil && item.Status == global.SUCCESS {
			successCount++
		}
	}

	switch {
	case successCount == len(filesInfo):
		return filesInfo, nil
	case successCount == 0:
		return filesInfo, e.NewBusinessError(e.FAILURE, "图片上传失败")
	default:
		return filesInfo, e.NewBusinessError(e.FileUploadPartialFail, partialImageUploadFailed)
	}
}

// IsPartialImageUploadError 判断是否属于部分图片上传失败错误。
func IsPartialImageUploadError(err error) bool {
	var businessErr *e.BusinessError
	return errors.As(err, &businessErr) && businessErr.GetCode() == e.FileUploadPartialFail
}
