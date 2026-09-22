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
	"gorm.io/gorm"
)

// CommonService 通用服务
type CommonService struct {
	Base
	db *gorm.DB
}

const maxUploadFileSize int64 = 10 * 1024 * 1024

// NewCommonService 创建通用服务实例。
func NewCommonService() *CommonService {
	return &CommonService{}
}

func (s *CommonService) SetDB(db *gorm.DB) *CommonService {
	s.db = db
	return s
}

// UploadImages 批量上传图片。
func (s CommonService) UploadImages(files []*multipart.FileHeader, path string) ([]*utils.FileInfo, error) {
	filesInfo := make([]*utils.FileInfo, 0, len(files))
	uploadErrs := make([]error, 0, len(files))
	for _, fileHeader := range files {
		file, err := s.UploadImage(fileHeader, true, path)
		if err != nil {
			log.Logger.Warn("文件上传失败",
				zap.String("filename", fileHeader.Filename),
				zap.Error(err),
			)
		}
		filesInfo = append(filesInfo, file)
		uploadErrs = append(uploadErrs, err)
	}

	return summarizeImageUploadResults(filesInfo, uploadErrs)
}

// UploadImage 上传单张图片并保存文件记录。
func (s CommonService) UploadImage(fileHeader *multipart.FileHeader, isPublic bool, path string) (*utils.FileInfo, error) {
	return s.UploadImageEx(fileHeader, isPublic, path, 0, "common")
}

// UploadImageEx 扩展上传单张图片，支持指定 FolderID 和 UploadScene
func (s CommonService) UploadImageEx(fileHeader *multipart.FileHeader, isPublic bool, path string, folderID uint, uploadScene string) (*utils.FileInfo, error) {
	fileInfo := initUploadResult(fileHeader)
	isPublicFlag := visibilityFlag(isPublic)
	basePath := storageBasePath(isPublic)
	uploadPath, err := normalizeUploadPath(path)
	if err != nil {
		return setFileFailure(fileInfo, "上传目录不合法", e.NewBusinessError(e.InvalidParameter, "上传目录不合法"))
	}
	if fileHeader.Size > maxUploadFileSize {
		return setFileFailure(fileInfo, "文件大小不能大于10M", e.NewBusinessError(e.FileTooLarge))
	}
	driver, storageConfig, activeDriver, err := NewActiveStorageDriver(context.Background())
	if err != nil {
		return setFileFailure(fileInfo, "存储配置异常", err)
	}
	uploadDir, err := resolveUploadDestination(basePath, uploadPath)
	if err != nil {
		return setFileFailure(fileInfo, "上传目录不合法", err)
	}

	result, err := utils.SaveUploadedImageWithUUID(fileHeader, uploadDir)
	if err != nil {
		if errors.Is(err, utils.ErrInvalidImageType) {
			return setFileFailure(fileInfo, "仅支持图片格式", e.NewBusinessError(e.FileMimeTypeNotAllowed))
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
	remoteObjectPending := false
	defer func() {
		if !remoteObjectPending {
			return
		}
		if deleteErr := driver.Delete(context.Background(), bucket, objectKey); deleteErr != nil {
			log.Logger.Warn("清理未提交的对象存储文件失败", zap.String("driver", activeDriver), zap.String("bucket", bucket), zap.String("object_key", objectKey), zap.Error(deleteErr))
		}
	}()
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
		remoteObjectPending = true
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
		OwnerUID:      s.GetAdminUserId(),
		IsPublic:      isPublicFlag,
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
	uploadFileModel.FolderID = folderID
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
	uploadFileModel.UploadScene = uploadScene
	uploadFileModel.UploadStatus = model.UploadStatusUploaded
	applyObjectToUploadFile(uploadFileModel, object)

	if err := uploadFileModel.Create(); err != nil {
		if !reused {
			_ = db.Delete(&model.UploadFileObject{}, object.ID).Error
		}
		if activeDriver == model.StorageDriverLocal && !reused {
			cleanupStoredUpload(savedPath)
		}
		return setFileFailure(fileInfo, "保存文件信息失败", err)
	}
	if activeDriver != model.StorageDriverLocal && !reused {
		remoteObjectPending = false
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

func (s CommonService) GetFileAccessPath(fileUUID string, variant string, checkAuth bool, currentUID uint) (FileAccessResult, error) {
	if len(fileUUID) != 32 {
		return FileAccessResult{}, e.NewBusinessError(e.FileIdentifierInvalid)
	}

	db, err := s.dbOrDefault()
	if err != nil {
		return FileAccessResult{}, err
	}

	uploadFile := model.NewUploadFiles()
	uploadFile.SetDB(db)
	// 通过UUID查询（更短，适合URL）
	err = db.Where("uuid = ?", fileUUID).First(uploadFile).Error
	if err != nil {
		return FileAccessResult{}, e.NewBusinessError(e.NotFound)
	}

	if !uploadFile.ExpiresAt.IsZero() && !uploadFile.ExpiresAt.Time.After(time.Now()) {
		return FileAccessResult{}, e.NewBusinessError(e.FileAccessDenied)
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
	objectMimeType := uploadFile.MimeType
	if uploadFile.FileObjectID > 0 {
		if db != nil {
			var object model.UploadFileObject
			if err := db.First(&object, uploadFile.FileObjectID).Error; err == nil {
				storageDriver = object.StorageDriver
				storageBase = object.StorageBase
				bucket = object.Bucket
				objectKey = firstNonEmpty(object.ObjectKey, object.StoragePath)
				objectMimeType = firstNonEmpty(object.MimeType, objectMimeType)
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
		if normalizeFileVariant(variant) == fileVariantThumb && isImageMimeType(firstNonEmpty(objectMimeType, uploadFile.MimeType)) {
			if thumbnailDriver, ok := driver.(filestorage.ImageThumbnailCapable); ok {
				signedThumbnailURL, thumbErr := thumbnailDriver.SignedImageThumbnailURL(context.Background(), bucket, objectKey, ttl)
				if thumbErr == nil {
					markFileAccessed(uploadFile)
					return FileAccessResult{RedirectURL: signedThumbnailURL}, nil
				}
				log.Logger.Warn("生成对象存储图片缩略图签名链接失败，回退原图", zap.String("driver", storageDriver), zap.String("uuid", uploadFile.UUID), zap.Error(thumbErr))
			}
		}
		signedURL, err := driver.SignedURL(context.Background(), bucket, objectKey, ttl)
		if err != nil {
			return FileAccessResult{}, e.NewBusinessError(e.FileAccessDenied)
		}
		markFileAccessed(uploadFile)
		return FileAccessResult{RedirectURL: signedURL}, nil
	}

	filePath, err := resolveUploadDestination(firstNonEmpty(storageBase, storageBasePath(uploadFile.IsPublic == global.Yes)), objectKey)
	if err != nil {
		return FileAccessResult{}, e.NewBusinessError(e.FileAccessDenied)
	}
	if normalizeFileVariant(variant) == fileVariantThumb && isImageMimeType(firstNonEmpty(objectMimeType, uploadFile.MimeType)) {
		thumbnailPath, thumbErr := ensureLocalImageThumbnail(filePath, firstNonEmpty(objectMimeType, uploadFile.MimeType))
		if thumbErr == nil {
			markFileAccessed(uploadFile)
			return FileAccessResult{LocalPath: thumbnailPath}, nil
		}
		log.Logger.Warn("生成文件缩略图失败，回退原图", zap.String("uuid", uploadFile.UUID), zap.Error(thumbErr))
	}
	markFileAccessed(uploadFile)
	return FileAccessResult{LocalPath: filePath}, nil
}

func (s CommonService) dbOrDefault() (*gorm.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	return model.GetDB()
}
