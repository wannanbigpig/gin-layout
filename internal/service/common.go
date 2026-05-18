package service

import (
	"context"
	"errors"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
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
	driver, storageConfig, activeDriver, err := NewActiveStorageDriver(context.Background())
	if err != nil {
		return setFileFailure(fileInfo, "存储配置异常", err)
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
	bucket := bucketForDriver(activeDriver, storageConfig, isPublicFlag)
	objectKey := result.Path
	etag := result.Sha256
	if activeDriver != model.StorageDriverLocal {
		file, err := os.Open(savedPath)
		if err != nil {
			cleanupStoredUpload(savedPath)
			return setFileFailure(fileInfo, "读取上传文件失败", err)
		}
		putResult, putErr := driver.Put(context.Background(), filestorage.PutInput{
			Bucket:      bucket,
			ObjectKey:   objectKey,
			Reader:      file,
			Size:        result.Size,
			ContentType: result.MimeType,
		})
		closeErr := file.Close()
		cleanupStoredUpload(savedPath)
		if putErr != nil {
			return setFileFailure(fileInfo, "保存到对象存储失败", putErr)
		}
		if closeErr != nil {
			return setFileFailure(fileInfo, "读取上传文件失败", closeErr)
		}
		bucket = putResult.Bucket
		objectKey = putResult.ObjectKey
		if putResult.ETag != "" {
			etag = putResult.ETag
		}
	}
	db, err := model.GetDB()
	if err != nil {
		if activeDriver == model.StorageDriverLocal {
			cleanupStoredUpload(savedPath)
		}
		return setFileFailure(fileInfo, "保存文件信息失败", err)
	}
	object, reused, err := ensureFileObject(db, uploadFileObjectInput{
		StorageDriver: activeDriver,
		StorageBase:   storageBaseForDriver(activeDriver, isPublic, bucket),
		Bucket:        bucket,
		StoragePath:   objectKey,
		ObjectKey:     objectKey,
		Size:          uint(result.Size),
		Hash:          result.Sha256,
		MimeType:      result.MimeType,
		ETag:          etag,
		Status:        model.StorageStatusStored,
	})
	if err != nil {
		if activeDriver == model.StorageDriverLocal {
			cleanupStoredUpload(savedPath)
		}
		return setFileFailure(fileInfo, "保存物理对象失败", err)
	}
	if activeDriver == model.StorageDriverLocal && reused {
		cleanupStoredUpload(savedPath)
	}

	uploadFileModel := model.NewUploadFiles()
	uploadFileModel.UID = s.GetAdminUserId()
	uploadFileModel.FolderID = 0
	uploadFileModel.LogicalPath = "/"
	uploadFileModel.DisplayName = result.OriginName
	uploadFileModel.OriginName = result.OriginName
	uploadFileModel.Name = result.Name
	uploadFileModel.Path = object.ObjectKey
	uploadFileModel.Size = uint(result.Size)
	uploadFileModel.Ext = result.Ext
	uploadFileModel.Hash = result.Sha256
	uploadFileModel.UUID = result.UUID
	uploadFileModel.MimeType = result.MimeType
	uploadFileModel.FileType = classifyUploadFileType(result.MimeType)
	uploadFileModel.IsPublic = isPublicFlag
	uploadFileModel.StorageDriver = activeDriver
	uploadFileModel.StorageBase = object.StorageBase
	uploadFileModel.Bucket = object.Bucket
	uploadFileModel.StoragePath = object.StoragePath
	uploadFileModel.ObjectKey = object.ObjectKey
	uploadFileModel.ETag = object.ETag
	uploadFileModel.StorageStatus = model.StorageStatusStored
	uploadFileModel.UploadSource = model.UploadSourceBackend
	uploadFileModel.UploadScene = "common"
	uploadFileModel.UploadStatus = model.UploadStatusUploaded
	applyObjectToUploadFile(uploadFileModel, object)

	if err := uploadFileModel.Create(); err != nil {
		if activeDriver == model.StorageDriverLocal && !reused {
			cleanupStoredUpload(savedPath)
			_ = db.Delete(&model.UploadFileObject{}, object.ID).Error
		}
		return setFileFailure(fileInfo, "保存文件信息失败", err)
	}

	fillFileInfoFromUploadResult(fileInfo, result)
	return fileInfo, nil
}

// GetFileAccessPath 获取文件访问路径
// fileUUID: 文件UUID（32位十六进制字符串，不带连字符），用于URL访问
// checkAuth: 是否检查权限（私有文件需要检查）
// currentUID: 当前用户ID（用于权限检查，0表示未登录）
type FileAccessResult struct {
	LocalPath   string
	RedirectURL string
}

func (s CommonService) GetFileAccessPath(fileUUID string, checkAuth bool, currentUID uint) (FileAccessResult, error) {
	if len(fileUUID) != 32 {
		return FileAccessResult{}, e.NewBusinessError(e.FileIdentifierInvalid)
	}

	uploadFile := model.NewUploadFiles()
	// 通过UUID查询（更短，适合URL）
	err := uploadFile.GetDetail("uuid = ?", fileUUID)
	if err != nil {
		return FileAccessResult{}, e.NewBusinessError(e.NotFound)
	}

	if uploadFile.IsPublic == global.No {
		if !checkAuth || currentUID == 0 {
			return FileAccessResult{}, e.NewBusinessError(e.FilePrivateAuthNeeded)
		}
		if uploadFile.UID != currentUID {
			return FileAccessResult{}, e.NewBusinessError(e.FileAccessDenied)
		}
	}

	storageDriver := uploadFile.StorageDriver
	storageBase := uploadFile.StorageBase
	bucket := uploadFile.Bucket
	objectKey := firstNonEmpty(uploadFile.ObjectKey, uploadFile.StoragePath, uploadFile.Path)
	if uploadFile.FileObjectID > 0 {
		if db, dbErr := model.GetDB(); dbErr == nil {
			var object model.UploadFileObject
			if err := db.First(&object, uploadFile.FileObjectID).Error; err == nil {
				storageDriver = object.StorageDriver
				storageBase = object.StorageBase
				bucket = object.Bucket
				objectKey = firstNonEmpty(object.ObjectKey, object.StoragePath)
			}
		}
	}

	if storageDriver != "" && storageDriver != model.StorageDriverLocal {
		driver, cfg, err := NewStorageDriverByName(context.Background(), storageDriver)
		if err != nil {
			return FileAccessResult{}, e.NewBusinessError(e.FileAccessDenied)
		}
		ttl := time.Duration(cfg.SignedURLTTLSeconds) * time.Second
		if ttl <= 0 {
			ttl = 5 * time.Minute
		}
		signedURL, err := driver.SignedURL(context.Background(), bucket, objectKey, ttl)
		if err != nil {
			return FileAccessResult{}, e.NewBusinessError(e.FileAccessDenied)
		}
		_ = uploadFile.UpdateById(uploadFile.ID, map[string]any{"last_accessed_at": time.Now()})
		return FileAccessResult{RedirectURL: signedURL}, nil
	}

	filePath, err := resolveUploadDestination(firstNonEmpty(storageBase, storageBasePath(uploadFile.IsPublic == global.Yes)), objectKey)
	if err != nil {
		return FileAccessResult{}, e.NewBusinessError(e.FileAccessDenied)
	}
	_ = uploadFile.UpdateById(uploadFile.ID, map[string]any{"last_accessed_at": time.Now()})
	return FileAccessResult{LocalPath: filePath}, nil
}
