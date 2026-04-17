package service

import (
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

const defaultUploadSubDir = "default"

func buildFileURL(uuid string) string {
	if uuid == "" {
		return ""
	}
	baseURL := strings.TrimSuffix(c.GetConfig().BaseURL, "/")
	if baseURL == "" {
		return "/admin/v1/file/" + uuid
	}
	return baseURL + "/admin/v1/file/" + uuid
}

func setFileFailure(info *utils.FileInfo, reason string, err error) (*utils.FileInfo, error) {
	info.FailureReason = reason
	info.Status = global.ERROR
	return info, err
}

func visibilityFlag(isPublic bool) uint8 {
	if isPublic {
		return global.Yes
	}
	return global.No
}

func storageBasePath(isPublic bool) string {
	cfg := c.GetConfig()
	if isPublic {
		return filepath.Join(cfg.BasePath, "storage/public")
	}
	return filepath.Join(cfg.BasePath, "storage/private")
}

func normalizeUploadPath(path string) (string, error) {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return defaultUploadSubDir, nil
	}
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	cleaned := filepath.Clean(normalized)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return defaultUploadSubDir, nil
	}
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("upload path must be relative")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("upload path escapes storage root")
	}
	return cleaned, nil
}

func resolveUploadDestination(basePath, uploadPath string) (string, error) {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("resolve storage base path: %w", err)
	}

	targetPath := filepath.Join(absBase, uploadPath)
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve upload target path: %w", err)
	}

	if absTarget != absBase && !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("upload target escapes storage root")
	}
	return absTarget, nil
}

func findReusableUploadFile(hash string, isPublic uint8) (*model.UploadFiles, error) {
	uploadFile := model.NewUploadFiles()
	if err := uploadFile.GetDetail("hash = ? AND is_public = ?", hash, isPublic); err != nil {
		return nil, err
	}
	return uploadFile, nil
}

func existingUploadFileExists(basePath, relativePath string) bool {
	absolutePath, err := resolveUploadDestination(basePath, relativePath)
	if err != nil {
		return false
	}
	_, err = os.Stat(absolutePath)
	return err == nil
}

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

func cleanupStoredUpload(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
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

func initUploadResult(fileHeader *multipart.FileHeader) *utils.FileInfo {
	return &utils.FileInfo{
		OriginName: fileHeader.Filename,
		Size:       fileHeader.Size,
		Ext:        filepath.Ext(fileHeader.Filename),
		Status:     global.ERROR,
	}
}
