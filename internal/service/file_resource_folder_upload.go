package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	dateutils "github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	fileutils "github.com/wannanbigpig/gin-layout/pkg/utils"
)

func (s *FileResourceService) FolderTree() ([]*resources.FileFolderResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	var folders []*model.UploadFileFolder
	if err := db.Order("sort ASC, id ASC").Find(&folders).Error; err != nil {
		return nil, err
	}
	stats, err := s.folderStats()
	if err != nil {
		return nil, err
	}
	transformer := resources.NewFileFolderTransformer()
	nodes := make(map[uint]*resources.FileFolderResources, len(folders))
	roots := make([]*resources.FileFolderResources, 0)
	for _, folder := range folders {
		node := transformer.ToStruct(folder)
		if stat, ok := stats[folder.ID]; ok {
			node.FileCount = stat.FileCount
			node.TotalSize = stat.TotalSize
		}
		nodes[folder.ID] = node
		if folder.ParentID == 0 {
			roots = append(roots, node)
		}
	}
	for _, folder := range folders {
		if folder.ParentID == 0 {
			continue
		}
		parent := nodes[folder.ParentID]
		if parent == nil {
			roots = append(roots, nodes[folder.ID])
			continue
		}
		parent.Children = append(parent.Children, nodes[folder.ID])
	}
	return roots, nil
}

func (s *FileResourceService) CreateFolder(params *form.FileFolderCreate, uid uint) (*resources.FileFolderResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	name, err := normalizeFolderName(params.Name)
	if err != nil {
		return nil, err
	}
	parentPath, err := s.folderLogicalPath(params.ParentID)
	if err != nil {
		return nil, err
	}
	if err := ensureFolderNameUnique(db, 0, params.ParentID, name); err != nil {
		return nil, err
	}
	folder := &model.UploadFileFolder{
		ParentID:    params.ParentID,
		Name:        name,
		LogicalPath: joinLogicalPath(parentPath, name),
		CreatedBy:   uid,
		UpdatedBy:   uid,
	}
	if err := db.Create(folder).Error; err != nil {
		return nil, err
	}
	return resources.NewFileFolderTransformer().ToStruct(folder), nil
}

func (s *FileResourceService) UpdateFolder(params *form.FileFolderUpdate, uid uint) (*resources.FileFolderResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	name, err := normalizeFolderName(params.Name)
	if err != nil {
		return nil, err
	}
	var updated model.UploadFileFolder
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		var folder model.UploadFileFolder
		if err := tx.First(&folder, params.ID).Error; err != nil {
			return err
		}
		if err := ensureFolderNameUnique(tx, folder.ID, folder.ParentID, name); err != nil {
			return err
		}
		oldPath := folder.LogicalPath
		parentPath, err := folderLogicalPathTx(tx, folder.ParentID)
		if err != nil {
			return err
		}
		folder.Name = name
		folder.LogicalPath = joinLogicalPath(parentPath, name)
		folder.UpdatedBy = uid
		if err := tx.Save(&folder).Error; err != nil {
			return err
		}
		if err := updateLogicalPathSnapshots(tx, folder.ID, oldPath, folder.LogicalPath); err != nil {
			return err
		}
		updated = folder
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resources.NewFileFolderTransformer().ToStruct(&updated), nil
}

func (s *FileResourceService) DeleteFolder(id uint) error {
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	var childCount int64
	if err := db.Model(&model.UploadFileFolder{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		return err
	}
	if childCount > 0 {
		return fmt.Errorf("folder is not empty")
	}
	var fileCount int64
	if err := db.Model(&model.UploadFiles{}).Where("folder_id = ?", id).Count(&fileCount).Error; err != nil {
		return err
	}
	if fileCount > 0 {
		return fmt.Errorf("folder is not empty")
	}
	result := db.Delete(&model.UploadFileFolder{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *FileResourceService) MoveFolder(params *form.FileFolderMove, uid uint) (*resources.FileFolderResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	targetParentID := params.ParentID
	if params.TargetParentID > 0 {
		targetParentID = params.TargetParentID
	}
	var moved model.UploadFileFolder
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		var folder model.UploadFileFolder
		if err := tx.First(&folder, params.ID).Error; err != nil {
			return err
		}
		if targetParentID == folder.ID {
			return fmt.Errorf("folder cannot move to itself")
		}
		if targetParentID > 0 {
			parentPath, err := folderLogicalPathTx(tx, targetParentID)
			if err != nil {
				return err
			}
			if parentPath == folder.LogicalPath || strings.HasPrefix(parentPath, folder.LogicalPath+"/") {
				return fmt.Errorf("folder cannot move to descendant")
			}
		}
		if err := ensureFolderNameUnique(tx, folder.ID, targetParentID, folder.Name); err != nil {
			return err
		}
		oldPath := folder.LogicalPath
		parentPath, err := folderLogicalPathTx(tx, targetParentID)
		if err != nil {
			return err
		}
		folder.ParentID = targetParentID
		folder.LogicalPath = joinLogicalPath(parentPath, folder.Name)
		folder.UpdatedBy = uid
		if err := tx.Save(&folder).Error; err != nil {
			return err
		}
		if err := updateLogicalPathSnapshots(tx, folder.ID, oldPath, folder.LogicalPath); err != nil {
			return err
		}
		moved = folder
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resources.NewFileFolderTransformer().ToStruct(&moved), nil
}

func (s *FileResourceService) MoveFiles(params *form.FileMove) (*resources.FileMoveResult, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	logicalPath, err := s.folderLogicalPath(params.FolderID)
	if err != nil {
		return nil, err
	}
	result := db.Model(&model.UploadFiles{}).Where("id IN ?", params.IDs).Updates(map[string]any{
		"folder_id":    params.FolderID,
		"logical_path": logicalPath,
		"updated_at":   time.Now(),
	})
	if result.Error != nil {
		return nil, result.Error
	}
	total := int64(len(params.IDs))
	return &resources.FileMoveResult{Total: total, Moved: result.RowsAffected, Skipped: total - result.RowsAffected}, nil
}

func (s *FileResourceService) UploadLocal(files []*multipart.FileHeader, params *form.FileLocalUpload, uid uint) ([]*resources.FileResourceResources, error) {
	items := make([]*resources.FileResourceResources, 0, len(files))
	for _, file := range files {
		item, err := s.uploadLocalOne(file, params, uid)
		if err != nil {
			return items, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *FileResourceService) UploadCredential(params *form.FileUploadCredential) (*resources.FileUploadCredentialResources, error) {
	_, cfg, activeDriver, err := s.activeStorageDriver(context.Background())
	if err != nil {
		return nil, err
	}
	if params.Driver != "" {
		activeDriver = params.Driver
	}
	if activeDriver == model.StorageDriverLocal {
		return nil, fmt.Errorf("local storage does not support direct upload")
	}
	bucket := bucketForDriver(activeDriver, cfg, params.IsPublic)
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	if object, err := findReusableFileObject(db, activeDriver, bucket, params.Hash); err == nil {
		return &resources.FileUploadCredentialResources{
			StorageDriver:   activeDriver,
			Driver:          activeDriver,
			Bucket:          object.Bucket,
			ObjectKey:       object.ObjectKey,
			Reuse:           true,
			FileObjectID:    object.ID,
			Size:            object.Size,
			Hash:            object.Hash,
			MimeType:        object.MimeType,
			ETag:            object.ETag,
			ObjectStatus:    object.Status,
			CompletePayload: buildCredentialCompletePayload(params, activeDriver, object.Bucket, object.ObjectKey, true, object.ID),
		}, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	driver, _, err := s.storageDriverByName(context.Background(), activeDriver)
	if err != nil {
		return nil, err
	}
	fileName := firstNonEmpty(params.FileName, params.OriginName)
	if strings.TrimSpace(fileName) == "" {
		return nil, fmt.Errorf("file name is required")
	}
	objectKey := buildUploadObjectKey(fileName)
	ttl := time.Duration(cfg.SignedURLTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	url, err := driver.PresignPutObject(context.Background(), bucket, objectKey, params.MimeType, ttl)
	if err != nil {
		return nil, err
	}
	return &resources.FileUploadCredentialResources{
		StorageDriver:   activeDriver,
		Driver:          activeDriver,
		Bucket:          bucket,
		ObjectKey:       objectKey,
		UploadURL:       url,
		URL:             url,
		Method:          "PUT",
		Headers:         map[string]string{"Content-Type": params.MimeType},
		ExpireAt:        dateutils.FormatDate{Time: time.Now().Add(ttl)},
		Reuse:           false,
		Size:            uint(params.Size),
		Hash:            params.Hash,
		MimeType:        params.MimeType,
		CompletePayload: buildCredentialCompletePayload(params, activeDriver, bucket, objectKey, false, 0),
	}, nil
}

func buildCredentialCompletePayload(params *form.FileUploadCredential, driver, bucket, objectKey string, reuse bool, fileObjectID uint) map[string]any {
	return map[string]any{
		"folder_id":      params.FolderID,
		"reuse":          reuse,
		"file_object_id": fileObjectID,
		"origin_name":    firstNonEmpty(params.OriginName, params.FileName),
		"display_name":   firstNonEmpty(params.OriginName, params.FileName),
		"name":           filepath.Base(objectKey),
		"size":           params.Size,
		"hash":           params.Hash,
		"mime_type":      params.MimeType,
		"is_public":      params.IsPublic,
		"storage_driver": driver,
		"driver":         driver,
		"bucket":         bucket,
		"object_key":     objectKey,
		"upload_scene":   params.UploadScene,
	}
}

func (s *FileResourceService) CompleteDirectUpload(params *form.FileUploadComplete, uid uint) (*resources.FileResourceResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	storageDriver := firstNonEmpty(params.StorageDriver, params.Driver)
	if storageDriver == "" {
		return nil, fmt.Errorf("storage driver is required")
	}
	if _, _, err := s.storageDriverByName(context.Background(), storageDriver); err != nil {
		return nil, err
	}
	logicalPath, err := s.folderLogicalPath(params.FolderID)
	if err != nil {
		return nil, err
	}
	var object *model.UploadFileObject
	if params.FileObjectID > 0 {
		object, err = findFileObjectByID(db, params.FileObjectID)
		if err != nil {
			return nil, err
		}
		if object.StorageDriver != storageDriver {
			return nil, fmt.Errorf("file object storage driver mismatch")
		}
	} else {
		if strings.TrimSpace(params.ObjectKey) == "" {
			return nil, fmt.Errorf("object_key is required")
		}
		object, _, err = ensureFileObject(db, uploadFileObjectInput{
			StorageDriver: storageDriver,
			StorageBase:   storageBaseForDriver(storageDriver, params.IsPublic == global.Yes, params.Bucket),
			Bucket:        params.Bucket,
			StoragePath:   params.ObjectKey,
			ObjectKey:     params.ObjectKey,
			Size:          params.Size,
			Hash:          params.Hash,
			MimeType:      params.MimeType,
			ETag:          params.ETag,
			Status:        model.StorageStatusStored,
		})
		if err != nil {
			return nil, err
		}
	}
	objectKey := firstNonEmpty(params.ObjectKey, object.ObjectKey)
	bucket := firstNonEmpty(params.Bucket, object.Bucket)
	size := params.Size
	if size == 0 {
		size = object.Size
	}
	hash := firstNonEmpty(params.Hash, object.Hash)
	mimeType := firstNonEmpty(params.MimeType, object.MimeType)
	etag := firstNonEmpty(params.ETag, object.ETag)
	fileUUID := params.UUID
	if fileUUID == "" {
		fileUUID = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	displayName := firstNonEmpty(params.DisplayName, params.OriginName)
	fileType, err := resolveUploadFileType(params.FileType, mimeType)
	if err != nil {
		return nil, err
	}
	uploadFile := &model.UploadFiles{
		UID:           uid,
		FolderID:      params.FolderID,
		LogicalPath:   logicalPath,
		DisplayName:   displayName,
		OriginName:    params.OriginName,
		Name:          firstNonEmpty(params.Name, filepath.Base(objectKey)),
		Path:          objectKey,
		Size:          size,
		Ext:           firstNonEmpty(params.Ext, filepath.Ext(params.OriginName), filepath.Ext(objectKey)),
		Hash:          hash,
		UUID:          fileUUID,
		MimeType:      mimeType,
		FileType:      fileType,
		IsPublic:      params.IsPublic,
		StorageDriver: storageDriver,
		StorageBase:   bucket,
		Bucket:        bucket,
		StoragePath:   objectKey,
		ObjectKey:     objectKey,
		ETag:          etag,
		StorageStatus: model.StorageStatusStored,
		UploadSource:  model.UploadSourceDirect,
		UploadScene:   params.UploadScene,
		UploadStatus:  model.UploadStatusUploaded,
	}
	applyObjectToUploadFile(uploadFile, object)
	if err := db.Create(uploadFile).Error; err != nil {
		return nil, err
	}
	s.fillObjectReuseCounts([]*model.UploadFiles{uploadFile})
	return resources.NewFileResourceTransformer().ToStruct(uploadFile), nil
}

func (s *FileResourceService) CreateFromReader(ctx context.Context, input ServerGeneratedFileInput) (*model.UploadFiles, error) {
	driver, cfg, activeDriver, err := s.activeStorageDriver(ctx)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(input.Reader)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	fileUUID := strings.ReplaceAll(uuid.NewString(), "-", "")
	ext := filepath.Ext(input.OriginName)
	objectKey := buildUploadObjectKey(fileUUID + ext)
	bucket := bucketForDriver(activeDriver, cfg, input.IsPublic)
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	object, reused, err := ensureFileObject(db, uploadFileObjectInput{
		StorageDriver: activeDriver,
		StorageBase:   storageBaseForDriver(activeDriver, input.IsPublic == global.Yes, bucket),
		Bucket:        bucket,
		StoragePath:   objectKey,
		ObjectKey:     objectKey,
		Size:          uint(len(data)),
		Hash:          hash,
		MimeType:      input.MimeType,
		ETag:          hash,
		Status:        model.StorageStatusStored,
	})
	if err != nil {
		return nil, err
	}
	if !reused {
		putResult, err := driver.Put(ctx, filestorage.PutInput{
			Bucket:      bucket,
			ObjectKey:   objectKey,
			Reader:      bytes.NewReader(data),
			Size:        int64(len(data)),
			ContentType: input.MimeType,
		})
		if err != nil {
			_ = db.Delete(&model.UploadFileObject{}, object.ID).Error
			return nil, err
		}
		updates := map[string]any{
			"bucket":       putResult.Bucket,
			"storage_path": putResult.ObjectKey,
			"object_key":   putResult.ObjectKey,
			"etag":         firstNonEmpty(putResult.ETag, hash),
			"updated_at":   time.Now(),
		}
		if err := db.Model(object).Updates(updates).Error; err != nil {
			return nil, err
		}
		object.Bucket = putResult.Bucket
		object.StoragePath = putResult.ObjectKey
		object.ObjectKey = putResult.ObjectKey
		object.ETag = firstNonEmpty(putResult.ETag, hash)
	}
	logicalPath, err := s.folderLogicalPath(input.FolderID)
	if err != nil {
		return nil, err
	}
	uploadFile := &model.UploadFiles{
		UID:           input.UID,
		FolderID:      input.FolderID,
		LogicalPath:   logicalPath,
		DisplayName:   firstNonEmpty(input.DisplayName, input.OriginName),
		OriginName:    input.OriginName,
		Name:          fileUUID + ext,
		Path:          object.ObjectKey,
		Size:          uint(len(data)),
		Ext:           ext,
		Hash:          hash,
		UUID:          fileUUID,
		MimeType:      input.MimeType,
		FileType:      classifyUploadFileType(input.MimeType),
		IsPublic:      input.IsPublic,
		StorageDriver: activeDriver,
		StorageBase:   object.StorageBase,
		Bucket:        object.Bucket,
		StoragePath:   object.StoragePath,
		ObjectKey:     object.ObjectKey,
		ETag:          object.ETag,
		StorageStatus: model.StorageStatusStored,
		UploadSource:  model.UploadSourceSystem,
		UploadScene:   input.UploadScene,
		UploadStatus:  model.UploadStatusUploaded,
	}
	applyObjectToUploadFile(uploadFile, object)
	if err := db.Create(uploadFile).Error; err != nil {
		return nil, err
	}
	return uploadFile, nil
}

type ServerGeneratedFileInput struct {
	UID         uint
	FolderID    uint
	OriginName  string
	DisplayName string
	MimeType    string
	IsPublic    uint8
	UploadScene string
	Reader      io.Reader
}

func (s *FileResourceService) CreateFromLocalPath(ctx context.Context, path string, input ServerGeneratedFileInput) (*model.UploadFiles, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if input.OriginName == "" {
		input.OriginName = filepath.Base(path)
	}
	if input.MimeType == "" {
		input.MimeType = mime.TypeByExtension(filepath.Ext(path))
	}
	input.Reader = file
	return s.CreateFromReader(ctx, input)
}

func (s *FileResourceService) uploadLocalOne(file *multipart.FileHeader, params *form.FileLocalUpload, uid uint) (*resources.FileResourceResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	logicalPath, err := s.folderLogicalPath(params.FolderID)
	if err != nil {
		return nil, err
	}
	basePath := storageBasePath(params.IsPublic == global.Yes)
	uploadDir, err := resolveUploadDestination(basePath, filepath.Join("file-resource", time.Now().Format("20060102")))
	if err != nil {
		return nil, err
	}
	result, err := fileutils.SaveUploadedFileWithUUID(file, uploadDir)
	if err != nil {
		return nil, err
	}
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		cleanupStoredUpload(result.Path)
		return nil, err
	}
	relPath, err := filepath.Rel(absBasePath, result.Path)
	if err != nil {
		cleanupStoredUpload(result.Path)
		return nil, err
	}
	relPath = filepath.ToSlash(relPath)
	bucket := bucketForDriver(model.StorageDriverLocal, filestorage.Config{}, params.IsPublic)
	object, reused, err := ensureFileObject(db, uploadFileObjectInput{
		StorageDriver: model.StorageDriverLocal,
		StorageBase:   basePath,
		Bucket:        bucket,
		StoragePath:   relPath,
		ObjectKey:     relPath,
		Size:          uint(result.Size),
		Hash:          result.Sha256,
		MimeType:      result.MimeType,
		ETag:          result.Sha256,
		Status:        model.StorageStatusStored,
	})
	if err != nil {
		cleanupStoredUpload(result.Path)
		return nil, err
	}
	if reused {
		cleanupStoredUpload(result.Path)
	}
	uploadFile := &model.UploadFiles{
		UID:           uid,
		FolderID:      params.FolderID,
		LogicalPath:   logicalPath,
		DisplayName:   result.OriginName,
		OriginName:    result.OriginName,
		Name:          result.Name,
		Path:          object.ObjectKey,
		Size:          uint(result.Size),
		Ext:           result.Ext,
		Hash:          result.Sha256,
		UUID:          result.UUID,
		MimeType:      result.MimeType,
		FileType:      classifyUploadFileType(result.MimeType),
		IsPublic:      params.IsPublic,
		StorageDriver: model.StorageDriverLocal,
		StorageBase:   object.StorageBase,
		Bucket:        object.Bucket,
		StoragePath:   object.StoragePath,
		ObjectKey:     object.ObjectKey,
		ETag:          object.ETag,
		StorageStatus: model.StorageStatusStored,
		UploadSource:  model.UploadSourceBackend,
		UploadScene:   params.UploadScene,
		UploadStatus:  model.UploadStatusUploaded,
	}
	applyObjectToUploadFile(uploadFile, object)
	if err := db.Create(uploadFile).Error; err != nil {
		if !reused {
			cleanupStoredUpload(result.Path)
			_ = db.Delete(&model.UploadFileObject{}, object.ID).Error
		}
		return nil, err
	}
	s.fillObjectReuseCounts([]*model.UploadFiles{uploadFile})
	return resources.NewFileResourceTransformer().ToStruct(uploadFile), nil
}

type folderStat struct {
	FolderID  uint
	FileCount int64
	TotalSize int64
}

func (s *FileResourceService) folderStats() (map[uint]folderStat, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	var rows []folderStat
	if err := db.Model(&model.UploadFiles{}).Select("folder_id, COUNT(*) AS file_count, COALESCE(SUM(size), 0) AS total_size").Group("folder_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]folderStat, len(rows))
	for _, row := range rows {
		result[row.FolderID] = row
	}
	return result, nil
}

func (s *FileResourceService) folderLogicalPath(folderID uint) (string, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return "", err
	}
	return folderLogicalPathTx(db, folderID)
}

func folderLogicalPathTx(tx *gorm.DB, folderID uint) (string, error) {
	if folderID == 0 {
		return "/", nil
	}
	var folder model.UploadFileFolder
	if err := tx.First(&folder, folderID).Error; err != nil {
		return "", err
	}
	return folder.LogicalPath, nil
}

func normalizeFolderName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || name == "." || name == ".." {
		return "", fmt.Errorf("invalid folder name")
	}
	return name, nil
}

func ensureFolderNameUnique(tx *gorm.DB, currentID, parentID uint, name string) error {
	query := tx.Model(&model.UploadFileFolder{}).Where("parent_id = ? AND name = ?", parentID, name)
	if currentID > 0 {
		query = query.Where("id <> ?", currentID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("folder name already exists")
	}
	return nil
}

func joinLogicalPath(parentPath, name string) string {
	parentPath = strings.TrimRight(strings.TrimSpace(parentPath), "/")
	if parentPath == "" {
		return "/" + name
	}
	return parentPath + "/" + name
}

func updateLogicalPathSnapshots(tx *gorm.DB, folderID uint, oldPath, newPath string) error {
	if err := tx.Model(&model.UploadFiles{}).Where("folder_id = ?", folderID).Update("logical_path", newPath).Error; err != nil {
		return err
	}
	var children []model.UploadFileFolder
	if err := tx.Where("logical_path LIKE ?", oldPath+"/%").Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		nextPath := newPath + strings.TrimPrefix(child.LogicalPath, oldPath)
		if err := tx.Model(&model.UploadFileFolder{}).Where("id = ?", child.ID).Update("logical_path", nextPath).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.UploadFiles{}).Where("folder_id = ?", child.ID).Update("logical_path", nextPath).Error; err != nil {
			return err
		}
	}
	return nil
}

func buildUploadObjectKey(filename string) string {
	ext := filepath.Ext(filename)
	return "uploads/" + time.Now().Format("20060102") + "/" + strings.ReplaceAll(uuid.NewString(), "-", "") + ext
}

// MultipartInit 分片上传初始化。
func (s *FileResourceService) MultipartInit(params *form.FileMultipartInit) (*resources.MultipartInitResource, error) {
	_, cfg, activeDriver, err := s.activeStorageDriver(context.Background())
	if err != nil {
		return nil, err
	}
	if params.Driver != "" {
		activeDriver = params.Driver
	}
	if activeDriver == model.StorageDriverLocal {
		return nil, fmt.Errorf("local storage does not support multipart upload")
	}
	bucket := bucketForDriver(activeDriver, cfg, params.IsPublic)
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	if object, err := findReusableFileObject(db, activeDriver, bucket, params.Hash); err == nil {
		return &resources.MultipartInitResource{
			UploadID:        "",
			Bucket:          object.Bucket,
			ObjectKey:       object.ObjectKey,
			Parts:           nil,
			Reuse:           true,
			FileObjectID:    object.ID,
			Size:            object.Size,
			Hash:            object.Hash,
			MimeType:        object.MimeType,
			ETag:            object.ETag,
			ObjectStatus:    object.Status,
			CompletePayload: buildMultipartCompletePayload(params, activeDriver, object.Bucket, object.ObjectKey, "", true, object),
		}, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	driver, _, err := s.storageDriverByName(context.Background(), activeDriver)
	if err != nil {
		return nil, err
	}
	objectKey := buildUploadObjectKey(params.OriginName)
	contentType := params.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	partCount := params.PartCount
	if partCount <= 0 {
		partCount = 1
	}
	initResult, err := driver.InitMultipartUpload(context.Background(), filestorage.MultipartUploadInput{
		Bucket:      bucket,
		ObjectKey:   objectKey,
		ContentType: contentType,
		PartCount:   partCount,
	})
	if err != nil {
		return nil, err
	}
	parts := make([]*resources.MultipartPartResource, 0, partCount)
	for i := 1; i <= partCount; i++ {
		partResult, err := driver.PresignMultipartPart(context.Background(), filestorage.MultipartPartInput{
			Bucket:     bucket,
			ObjectKey:  objectKey,
			UploadID:   initResult.UploadID,
			PartNumber: int32(i),
		})
		if err != nil {
			_ = driver.AbortMultipartUpload(context.Background(), bucket, objectKey, initResult.UploadID)
			return nil, err
		}
		parts = append(parts, &resources.MultipartPartResource{
			PartNumber: partResult.PartNumber,
			UploadURL:  partResult.UploadURL,
			Headers:    partResult.Headers,
		})
	}
	return &resources.MultipartInitResource{
		UploadID:        initResult.UploadID,
		Bucket:          initResult.Bucket,
		ObjectKey:       initResult.ObjectKey,
		Parts:           parts,
		Reuse:           false,
		Size:            uint(params.Size),
		Hash:            params.Hash,
		MimeType:        params.MimeType,
		CompletePayload: buildMultipartCompletePayload(params, activeDriver, initResult.Bucket, initResult.ObjectKey, initResult.UploadID, false, nil),
	}, nil
}

// MultipartComplete 分片上传完成。
func (s *FileResourceService) MultipartComplete(params *form.FileMultipartComplete, uid uint) (*resources.FileResourceResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	storageDriver := firstNonEmpty(params.Driver)
	var object *model.UploadFileObject
	if shouldResolveMultipartReuse(params) {
		object, err = s.resolveReusableMultipartObject(db, params, storageDriver)
		if err != nil {
			return nil, err
		}
		if storageDriver == "" {
			storageDriver = object.StorageDriver
		}
	} else {
		if storageDriver == "" {
			return nil, fmt.Errorf("storage driver is required")
		}
		if strings.TrimSpace(params.UploadID) == "" {
			return nil, fmt.Errorf("upload_id is required")
		}
		if strings.TrimSpace(params.Bucket) == "" {
			return nil, fmt.Errorf("bucket is required")
		}
		if strings.TrimSpace(params.ObjectKey) == "" {
			return nil, fmt.Errorf("object_key is required")
		}
		completeParts, err := validateMultipartCompleteParts(params.Parts)
		if err != nil {
			return nil, err
		}
		driver, _, err := s.storageDriverByName(context.Background(), storageDriver)
		if err != nil {
			return nil, err
		}
		if err := driver.CompleteMultipartUpload(context.Background(), params.Bucket, params.ObjectKey, params.UploadID, completeParts); err != nil {
			return nil, err
		}
		object, _, err = ensureFileObject(db, uploadFileObjectInput{
			StorageDriver: storageDriver,
			StorageBase:   storageBaseForDriver(storageDriver, params.IsPublic == global.Yes, params.Bucket),
			Bucket:        params.Bucket,
			StoragePath:   params.ObjectKey,
			ObjectKey:     params.ObjectKey,
			Size:          params.Size,
			Hash:          params.Hash,
			MimeType:      params.MimeType,
			ETag:          "",
			Status:        model.StorageStatusStored,
		})
		if err != nil {
			return nil, err
		}
	}
	logicalPath, err := s.folderLogicalPath(params.FolderID)
	if err != nil {
		return nil, err
	}
	bucket := firstNonEmpty(params.Bucket, object.Bucket)
	objectKey := firstNonEmpty(params.ObjectKey, object.ObjectKey)
	size := params.Size
	if size == 0 {
		size = object.Size
	}
	hash := firstNonEmpty(params.Hash, object.Hash)
	mimeType := firstNonEmpty(params.MimeType, object.MimeType)
	fileUUID := strings.ReplaceAll(uuid.NewString(), "-", "")
	displayName := params.OriginName
	fileType, err := resolveUploadFileType(params.FileType, mimeType)
	if err != nil {
		return nil, err
	}
	uploadFile := &model.UploadFiles{
		UID:           uid,
		FolderID:      params.FolderID,
		LogicalPath:   logicalPath,
		DisplayName:   displayName,
		OriginName:    params.OriginName,
		Name:          filepath.Base(objectKey),
		Path:          objectKey,
		Size:          size,
		Ext:           filepath.Ext(params.OriginName),
		Hash:          hash,
		UUID:          fileUUID,
		MimeType:      mimeType,
		FileType:      fileType,
		IsPublic:      params.IsPublic,
		StorageDriver: storageDriver,
		StorageBase:   storageBaseForDriver(storageDriver, params.IsPublic == global.Yes, bucket),
		Bucket:        bucket,
		StoragePath:   objectKey,
		ObjectKey:     objectKey,
		ETag:          "",
		StorageStatus: model.StorageStatusStored,
		UploadSource:  model.UploadSourceDirect,
		UploadStatus:  model.UploadStatusUploaded,
	}
	applyObjectToUploadFile(uploadFile, object)
	if err := db.Create(uploadFile).Error; err != nil {
		return nil, err
	}
	s.fillObjectReuseCounts([]*model.UploadFiles{uploadFile})
	return resources.NewFileResourceTransformer().ToStruct(uploadFile), nil
}

func resolveUploadFileType(explicit, mimeType string) (string, error) {
	fileType := strings.TrimSpace(explicit)
	if fileType == "" {
		return classifyUploadFileType(mimeType), nil
	}
	if strings.Contains(fileType, ",") {
		return "", fmt.Errorf("file_type must be a single value")
	}
	if !isAllowedUploadFileType(fileType) {
		return "", fmt.Errorf("unsupported file_type: %s", fileType)
	}
	return fileType, nil
}

func isAllowedUploadFileType(fileType string) bool {
	switch fileType {
	case "image", "pdf", "word", "excel", "ppt", "archive", "text", "audio", "video", "other":
		return true
	default:
		return false
	}
}

func buildMultipartCompletePayload(params *form.FileMultipartInit, driver, bucket, objectKey, uploadID string, reuse bool, object *model.UploadFileObject) map[string]any {
	fileObjectID := uint(0)
	size := uint(params.Size)
	hash := params.Hash
	mimeType := params.MimeType
	etag := ""
	if object != nil {
		fileObjectID = object.ID
		size = object.Size
		hash = object.Hash
		mimeType = object.MimeType
		etag = object.ETag
	}
	return map[string]any{
		"upload_id":      uploadID,
		"reuse":          reuse,
		"file_object_id": fileObjectID,
		"bucket":         bucket,
		"object_key":     objectKey,
		"origin_name":    params.OriginName,
		"size":           size,
		"hash":           hash,
		"mime_type":      mimeType,
		"is_public":      params.IsPublic,
		"driver":         driver,
		"etag":           etag,
		"parts":          []any{},
	}
}

func shouldResolveMultipartReuse(params *form.FileMultipartComplete) bool {
	if params == nil {
		return false
	}
	if params.Reuse || params.FileObjectID > 0 {
		return true
	}
	// 兼容现有前端 reuse 分支：multipart init 命中已存在对象时，complete 只回传 bucket/object_key/hash/driver，
	// 不会再显式带 reuse/file_object_id/upload_id/parts。
	return strings.TrimSpace(params.UploadID) == "" && len(params.Parts) == 0 && strings.TrimSpace(params.Hash) != ""
}

func (s *FileResourceService) resolveReusableMultipartObject(db *gorm.DB, params *form.FileMultipartComplete, storageDriver string) (*model.UploadFileObject, error) {
	if params.FileObjectID > 0 {
		object, err := findFileObjectByID(db, params.FileObjectID)
		if err != nil {
			return nil, err
		}
		if storageDriver != "" && object.StorageDriver != storageDriver {
			return nil, fmt.Errorf("file object storage driver mismatch")
		}
		return object, nil
	}
	if storageDriver == "" {
		return nil, fmt.Errorf("storage driver is required")
	}
	object, err := findReusableFileObject(db, storageDriver, params.Bucket, params.Hash)
	if err != nil {
		return nil, err
	}
	return object, nil
}

func validateMultipartCompleteParts(parts []form.MultipartCompletePart) ([]filestorage.CompletePartInfo, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("parts is required")
	}
	completeParts := make([]filestorage.CompletePartInfo, 0, len(parts))
	seen := make(map[int32]struct{}, len(parts))
	for _, part := range parts {
		etag := strings.TrimSpace(part.ETag)
		if part.PartNumber <= 0 {
			return nil, fmt.Errorf("part_number must be greater than 0")
		}
		if etag == "" {
			return nil, fmt.Errorf("etag is required")
		}
		if _, ok := seen[part.PartNumber]; ok {
			return nil, fmt.Errorf("duplicate part_number: %d", part.PartNumber)
		}
		seen[part.PartNumber] = struct{}{}
		completeParts = append(completeParts, filestorage.CompletePartInfo{
			PartNumber: part.PartNumber,
			ETag:       etag,
		})
	}
	sort.Slice(completeParts, func(i, j int) bool {
		return completeParts[i].PartNumber < completeParts[j].PartNumber
	})
	for i, part := range completeParts {
		expected := int32(i + 1)
		if part.PartNumber != expected {
			return nil, fmt.Errorf("part_number must be continuous from 1")
		}
	}
	return completeParts, nil
}

// MultipartAbort 分片上传中止。
func (s *FileResourceService) MultipartAbort(params *form.FileMultipartAbort) error {
	storageDriver := firstNonEmpty(params.Driver)
	if storageDriver == "" {
		return fmt.Errorf("storage driver is required")
	}
	driver, _, err := s.storageDriverByName(context.Background(), storageDriver)
	if err != nil {
		return err
	}
	return driver.AbortMultipartUpload(context.Background(), params.Bucket, params.ObjectKey, params.UploadID)
}
