package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"

	"uuid"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	pkgErrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	dateutils "github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	fileutils "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// FolderStatsResult 目录统计信息（供删除前确认使用）。
type FolderStatsResult struct {
	FileCount        int64 `json:"file_count"`
	ChildFolderCount int64 `json:"child_folder_count"`
	TotalSize        int64 `json:"total_size"`
}

func (s *FileResourceService) FolderTree(includeHidden bool) ([]*resources.FileFolderResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	query := db.Order("sort ASC, id ASC")
	if !includeHidden {
		query = query.Where("is_hidden = 0")
	}
	var folders []*model.UploadFileFolder
	if err := query.Find(&folders).Error; err != nil {
		return nil, err
	}
	stats, err := s.folderStats()
	if err != nil {
		return nil, err
	}
	creatorNames := s.queryFolderCreatorNames(folders)
	transformer := resources.NewFileFolderTransformer()
	nodes := make(map[uint]*resources.FileFolderResources, len(folders))
	roots := make([]*resources.FileFolderResources, 0)
	for _, folder := range folders {
		node := transformer.ToStruct(folder)
		if stat, ok := stats[folder.ID]; ok {
			node.FileCount = stat.FileCount
			node.TotalSize = stat.TotalSize
		}
		fillFolderCreatorName(node, creatorNames)
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
	return s.buildFolderResource(folder), nil
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
	return s.buildFolderResource(&updated), nil
}

// FolderStats 查询目录下的文件数和子目录数（供删除前确认使用）。
func (s *FileResourceService) FolderStats(id uint) (*FolderStatsResult, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	var folder model.UploadFileFolder
	if err := db.First(&folder, id).Error; err != nil {
		return nil, pkgErrors.NewBusinessError(pkgErrors.NotFound)
	}
	allFolderIDs, err := collectDescendantFolderIDs(db, id)
	if err != nil {
		return nil, err
	}
	var fileCount int64
	if err := db.Model(&model.UploadFiles{}).Where("folder_id IN ?", allFolderIDs).Count(&fileCount).Error; err != nil {
		return nil, err
	}
	var totalSize int64
	if err := db.Model(&model.UploadFiles{}).Where("folder_id IN ?", allFolderIDs).Select("COALESCE(SUM(size), 0)").Scan(&totalSize).Error; err != nil {
		return nil, err
	}
	childFolderCount := int64(len(allFolderIDs) - 1)
	return &FolderStatsResult{FileCount: fileCount, ChildFolderCount: childFolderCount, TotalSize: totalSize}, nil
}

// DeleteFolder 级联删除目录及其所有子目录和文件。
// 子目录递归删除，文件走软删除（进入回收站）。
func (s *FileResourceService) DeleteFolder(id uint, deletedBy uint, reason string) error {
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	return access.RunInTransaction(db, func(tx *gorm.DB) error {
		var folder model.UploadFileFolder
		if err := tx.First(&folder, id).Error; err != nil {
			return pkgErrors.NewBusinessError(pkgErrors.NotFound)
		}
		allFolderIDs, err := collectDescendantFolderIDs(tx, id)
		if err != nil {
			return err
		}
		if len(allFolderIDs) > 0 {
			var fileIDs []uint
			if err := tx.Model(&model.UploadFiles{}).Where("folder_id IN ?", allFolderIDs).Pluck("id", &fileIDs).Error; err != nil {
				return err
			}
			if len(fileIDs) > 0 {
				var refs []*model.UploadFileReference
				if err := tx.Where("file_id IN ?", fileIDs).Find(&refs).Error; err != nil {
					return err
				}
				if len(refs) > 0 {
					return NewFileReferencedDeleteError(buildFileReferenceResources(refs))
				}
			}
			updates := map[string]any{
				"deleted_at":     time.Now().Unix(),
				"deleted_by":     deletedBy,
				"deleted_reason": reason,
			}
			if err := tx.Model(&model.UploadFiles{}).Where("folder_id IN ?", allFolderIDs).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&model.UploadFileFolder{}, allFolderIDs).Error; err != nil {
			return err
		}
		return nil
	})
}

// collectDescendantFolderIDs 递归收集目录及其所有后代目录 ID。
func collectDescendantFolderIDs(db *gorm.DB, rootID uint) ([]uint, error) {
	ids := []uint{rootID}
	current := []uint{rootID}
	for len(current) > 0 {
		var children []uint
		if err := db.Model(&model.UploadFileFolder{}).Where("parent_id IN ?", current).Pluck("id", &children).Error; err != nil {
			return nil, err
		}
		if len(children) == 0 {
			break
		}
		ids = append(ids, children...)
		current = children
	}
	return ids, nil
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
			return pkgErrors.NewBusinessError(pkgErrors.FolderMoveInvalid)
		}
		if targetParentID > 0 {
			parentPath, err := folderLogicalPathTx(tx, targetParentID)
			if err != nil {
				return err
			}
			if parentPath == folder.LogicalPath || strings.HasPrefix(parentPath, folder.LogicalPath+"/") {
				return pkgErrors.NewBusinessError(pkgErrors.FolderMoveInvalid)
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
	return s.buildFolderResource(&moved), nil
}

func (s *FileResourceService) buildFolderResource(folder *model.UploadFileFolder) *resources.FileFolderResources {
	node := resources.NewFileFolderTransformer().ToStruct(folder)
	fillFolderCreatorName(node, s.queryFolderCreatorNames([]*model.UploadFileFolder{folder}))
	return node
}

func (s *FileResourceService) queryFolderCreatorNames(folders []*model.UploadFileFolder) map[uint]uploaderInfo {
	if len(folders) == 0 {
		return nil
	}
	uidSet := make(map[uint]struct{})
	uids := make([]uint, 0, len(folders))
	for _, folder := range folders {
		if folder == nil || folder.CreatedBy == 0 {
			continue
		}
		if _, exists := uidSet[folder.CreatedBy]; exists {
			continue
		}
		uidSet[folder.CreatedBy] = struct{}{}
		uids = append(uids, folder.CreatedBy)
	}
	return s.queryUploaderNames(uids)
}

func fillFolderCreatorName(node *resources.FileFolderResources, creatorNames map[uint]uploaderInfo) {
	if node == nil || node.CreatedBy == 0 {
		return
	}
	info, exists := creatorNames[node.CreatedBy]
	if !exists {
		return
	}
	node.CreatorName = info.Nickname
	node.CreatorUsername = info.Username
	node.UploaderName = info.Nickname
	node.UploaderUsername = info.Username
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
	// 逐文件移动：唯一索引 uniq_folder_name_deleted 下，目标目录已有同名活跃记录时单条 UPDATE 会冲突，
	// 跳过该文件而非让整批失败。整体包裹在事务中保证原子性。
	var moved int64
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		for _, id := range params.IDs {
			res := tx.Model(&model.UploadFiles{}).Where("id = ?", id).Updates(map[string]any{
				"folder_id":    params.FolderID,
				"logical_path": logicalPath,
				"updated_at":   time.Now(),
			})
			if res.Error != nil {
				if isUploadNameDuplicateErr(res.Error) {
					continue // 目标目录已存在同名文件，跳过
				}
				return res.Error
			}
			moved += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	total := int64(len(params.IDs))
	return &resources.FileMoveResult{Total: total, Moved: moved, Skipped: total - moved}, nil
}

func (s *FileResourceService) UploadLocal(files []*multipart.FileHeader, params *form.FileLocalUpload, uid uint) (*resources.FileLocalUploadBatchResult, error) {
	result := &resources.FileLocalUploadBatchResult{
		Total: int64(len(files)),
		Items: make([]*resources.FileResourceResources, 0, len(files)),
	}
	for idx, file := range files {
		item, err := s.uploadLocalOne(file, params, uid)
		if err != nil {
			if len(files) <= 1 {
				return nil, err
			}
			result.Failures = append(result.Failures, buildLocalUploadFailureItem(idx, file, err))
			continue
		}
		result.Items = append(result.Items, item)
	}
	result.Success = int64(len(result.Items))
	result.Failed = int64(len(result.Failures))
	return result, nil
}

func buildLocalUploadFailureItem(index int, file *multipart.FileHeader, err error) *resources.FileUploadBatchFailure {
	item := &resources.FileUploadBatchFailure{
		Index:   index,
		Message: err.Error(),
	}
	if file != nil {
		item.OriginName = file.Filename
	}
	var businessErr *pkgErrors.BusinessError
	if errors.As(err, &businessErr) {
		item.Code = businessErr.GetCode()
		item.Message = businessErr.GetMessage()
	}
	return item
}

func (s *FileResourceService) UploadCredential(params *form.FileUploadCredential, uidArg ...uint) (*resources.FileUploadCredentialResources, error) {
	uid := firstUint(uidArg...)
	fileName := firstNonEmpty(params.FileName, params.OriginName)
	if err := s.validateFileLimit(fileName, params.Size, params.MimeType); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Hash) == "" {
		return nil, pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "hash is required")
	}
	if params.Size <= 0 {
		return nil, pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "size is required")
	}
	if strings.TrimSpace(params.MimeType) == "" {
		return nil, pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "mime_type is required")
	}
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
	ttl := time.Duration(cfg.SignedURLTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if object, err := findReusableFileObject(db, activeDriver, bucket, params.Hash, uid, params.IsPublic); err == nil {
		completeToken, _, err := createUploadSession(db, uploadSessionInput{
			UploadType:    model.UploadSessionTypeDirect,
			StorageDriver: activeDriver,
			StorageBase:   object.StorageBase,
			Bucket:        object.Bucket,
			StoragePath:   object.StoragePath,
			ObjectKey:     object.ObjectKey,
			FileObjectID:  object.ID,
			OwnerUID:      uid,
			FolderID:      params.FolderID,
			OriginName:    firstNonEmpty(params.OriginName, params.FileName),
			Size:          object.Size,
			Hash:          object.Hash,
			MimeType:      object.MimeType,
			ETag:          object.ETag,
			IsPublic:      params.IsPublic,
			UploadScene:   params.UploadScene,
			ExpiresAt:     time.Now().Add(ttl),
		})
		if err != nil {
			return nil, err
		}
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
			CompleteToken:   completeToken,
			CompletePayload: buildCredentialCompletePayload(completeToken),
		}, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	driver, _, err := s.storageDriverByName(context.Background(), activeDriver)
	if err != nil {
		return nil, err
	}
	objectKey := buildUploadObjectKey(fileName)
	url, err := driver.PresignPutObject(context.Background(), bucket, objectKey, params.MimeType, ttl)
	if err != nil {
		return nil, err
	}
	completeToken, _, err := createUploadSession(db, uploadSessionInput{
		UploadType:    model.UploadSessionTypeDirect,
		StorageDriver: activeDriver,
		Bucket:        bucket,
		ObjectKey:     objectKey,
		OwnerUID:      uid,
		FolderID:      params.FolderID,
		OriginName:    firstNonEmpty(params.OriginName, params.FileName),
		Size:          uint(params.Size),
		Hash:          params.Hash,
		MimeType:      params.MimeType,
		IsPublic:      params.IsPublic,
		UploadScene:   params.UploadScene,
		ExpiresAt:     time.Now().Add(ttl),
	})
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
		CompleteToken:   completeToken,
		CompletePayload: buildCredentialCompletePayload(completeToken),
	}, nil
}

func buildCredentialCompletePayload(completeToken string) map[string]any {
	return map[string]any{
		"complete_token": completeToken,
	}
}

func normalizeCredentialItem(parent *form.FileUploadCredentialBatch, item form.FileUploadCredentialBatchItem) *form.FileUploadCredential {
	isPublic := parent.IsPublic
	if item.IsPublic != nil {
		isPublic = *item.IsPublic
	}
	return &form.FileUploadCredential{
		FolderID:    item.FolderID,
		FileName:    item.FileName,
		OriginName:  item.OriginName,
		MimeType:    item.MimeType,
		Size:        item.Size,
		Hash:        item.Hash,
		IsPublic:    isPublic,
		UploadScene: firstNonEmpty(item.UploadScene, parent.UploadScene),
		Driver:      firstNonEmpty(item.Driver, parent.Driver),
	}
}

func buildBatchFailure(index int, clientID string, err error) *resources.FileUploadBatchFailure {
	failure := &resources.FileUploadBatchFailure{
		ClientID: clientID,
		Index:    index,
		Message:  err.Error(),
	}
	var businessErr *pkgErrors.BusinessError
	if errors.As(err, &businessErr) {
		failure.Code = businessErr.GetCode()
		failure.Message = businessErr.GetMessage()
	}
	return failure
}

const uploadBatchConcurrency = 8

func (s *FileResourceService) UploadCredentialBatch(params *form.FileUploadCredentialBatch, uidArg ...uint) (*resources.FileUploadCredentialBatchResult, error) {
	uid := firstUint(uidArg...)
	total := len(params.Items)
	result := &resources.FileUploadCredentialBatchResult{
		Total: int64(total),
		Items: make([]*resources.FileUploadCredentialBatchItemResult, total),
	}
	if total == 0 {
		return result, nil
	}
	concurrency := uploadBatchConcurrency
	if concurrency > total {
		concurrency = total
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success, failed := 0, 0
	for index, item := range params.Items {
		wg.Add(1)
		sem <- struct{}{}
		go func(index int, it form.FileUploadCredentialBatchItem) {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := s.UploadCredential(normalizeCredentialItem(params, it), uid)
			var itemResult *resources.FileUploadCredentialBatchItemResult
			if err != nil {
				itemResult = &resources.FileUploadCredentialBatchItemResult{
					ClientID: it.ClientID,
					Success:  false,
					Error:    buildBatchFailure(index, it.ClientID, err),
				}
			} else {
				itemResult = &resources.FileUploadCredentialBatchItemResult{
					ClientID: it.ClientID,
					Success:  true,
					Data:     data,
				}
			}
			mu.Lock()
			result.Items[index] = itemResult
			if itemResult.Success {
				success++
			} else {
				failed++
			}
			mu.Unlock()
		}(index, item)
	}
	wg.Wait()
	result.Success = int64(success)
	result.Failed = int64(failed)
	return result, nil
}

func (s *FileResourceService) CompleteDirectUpload(params *form.FileUploadComplete, uid uint) (*resources.FileResourceResources, error) {
	return s.CompleteDirectUploadWithContext(context.Background(), params, uid)
}

func (s *FileResourceService) CompleteDirectUploadWithContext(ctx context.Context, params *form.FileUploadComplete, uid uint) (*resources.FileResourceResources, error) {
	return s.completeDirectUploadWithPath(ctx, params, uid, nil)
}

func (s *FileResourceService) completeDirectUploadWithPath(ctx context.Context, params *form.FileUploadComplete, uid uint, pathResolver func(uint) (string, error)) (*resources.FileResourceResources, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	sniffTarget, err := s.resolveDirectUploadMimeSniffTarget(ctx, params, uid)
	if err != nil {
		return nil, err
	}
	if err := s.validateUploadObjectMime(ctx, "direct upload", sniffTarget.storageDriver, sniffTarget.bucket, sniffTarget.objectKey, sniffTarget.mimeType); err != nil {
		return nil, err
	}
	// 事务外校验实际文件大小：防止攻击者伪造小 size 上传超限文件
	if err := s.validateActualObjectSize(ctx, sniffTarget.storageDriver, sniffTarget.bucket, sniffTarget.objectKey, sniffTarget.declaredSize); err != nil {
		return nil, err
	}
	var uploadFile *model.UploadFiles
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		session, err := findPendingUploadSession(tx, params.CompleteToken, model.UploadSessionTypeDirect, uid)
		if err != nil {
			return err
		}
		if err := validateDirectCompleteSession(session, params, uid); err != nil {
			return err
		}
		if err := requireUploadSessionObject(session); err != nil {
			return err
		}
		storageDriver := session.StorageDriver
		if _, _, err := s.storageDriverByName(context.Background(), storageDriver); err != nil {
			return err
		}
		var logicalPath string
		if pathResolver != nil {
			logicalPath, err = pathResolver(session.FolderID)
		} else {
			logicalPath, err = folderLogicalPathTx(tx, session.FolderID)
		}
		if err != nil {
			return err
		}
		var object *model.UploadFileObject
		var reused bool
		if session.FileObjectID > 0 {
			object, err = findFileObjectByID(tx, session.FileObjectID)
			if err != nil {
				return err
			}
			if err := validateFileObjectMatchesSession(object, session); err != nil {
				return err
			}
		} else {
			object, reused, err = ensureFileObject(tx, uploadFileObjectInput{
				StorageDriver: storageDriver,
				StorageBase:   storageBaseForDriver(storageDriver, session.IsPublic == global.Yes, session.Bucket),
				Bucket:        session.Bucket,
				StoragePath:   session.ObjectKey,
				ObjectKey:     session.ObjectKey,
				OwnerUID:      uid,
				IsPublic:      session.IsPublic,
				Size:          session.Size,
				Hash:          session.Hash,
				MimeType:      session.MimeType,
				ETag:          params.ETag,
				Status:        model.StorageStatusStored,
			})
			if err != nil {
				return err
			}
		}
		objectKey := object.ObjectKey
		bucket := object.Bucket
		size := session.Size
		hash := session.Hash
		mimeType := session.MimeType
		etag := firstNonEmpty(params.ETag, object.ETag)
		fileUUID := params.UUID
		if fileUUID == "" {
			fileUUID = strings.ReplaceAll(uuid.New().String(), "-", "")
		}

		fileType, err := resolveUploadFileType(params.FileType, mimeType)
		if err != nil {
			return err
		}

		created, existing, err := s.createUploadFileWithConflict(tx, session.FolderID, session.OriginName, hash, uid, session.IsPublic, func(resolved *uploadFileConflictResult) *model.UploadFiles {
			originName := resolved.FinalName
			uf := &model.UploadFiles{
				UID:           uid,
				FolderID:      session.FolderID,
				LogicalPath:   logicalPath,
				DisplayName:   firstNonEmpty(params.DisplayName, originName),
				OriginName:    originName,
				Name:          firstNonEmpty(params.Name, filepath.Base(objectKey)),
				Path:          objectKey,
				Size:          size,
				Ext:           firstNonEmpty(params.Ext, filepath.Ext(originName), filepath.Ext(objectKey)),
				Hash:          hash,
				UUID:          fileUUID,
				MimeType:      mimeType,
				FileType:      fileType,
				IsPublic:      session.IsPublic,
				StorageDriver: storageDriver,
				StorageBase:   bucket,
				Bucket:        bucket,
				StoragePath:   objectKey,
				ObjectKey:     objectKey,
				ETag:          etag,
				StorageStatus: model.StorageStatusStored,
				UploadSource:  model.UploadSourceDirect,
				UploadScene:   session.UploadScene,
				UploadStatus:  model.UploadStatusUploaded,
			}
			applyObjectToUploadFile(uf, object)
			return uf
		})
		if err != nil {
			return err
		}
		if err := markUploadSessionCompleted(tx, session, created.ID, object.ID); err != nil {
			return err
		}
		if existing && session.FileObjectID == 0 && !reused {
			if delErr := tx.Delete(&model.UploadFileObject{}, object.ID).Error; delErr != nil {
				log.Warn("清理孤儿 UploadFileObject 失败", zap.Uint("object_id", object.ID), zap.Error(delErr))
			}
		}
		uploadFile = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.fillObjectReuseCounts([]*model.UploadFiles{uploadFile})
	return resources.NewFileResourceTransformer().ToStruct(uploadFile), nil
}

func normalizeCompleteItem(parent *form.FileUploadCompleteBatch, item form.FileUploadCompleteBatchItem) *form.FileUploadComplete {
	isPublic := parent.IsPublic
	if item.IsPublic != nil {
		isPublic = *item.IsPublic
	}
	driver := firstNonEmpty(item.Driver, item.StorageDriver, parent.Driver)
	return &form.FileUploadComplete{
		CompleteToken: item.CompleteToken,
		FolderID:      item.FolderID,
		Reuse:         item.Reuse,
		FileObjectID:  item.FileObjectID,
		OriginName:    item.OriginName,
		DisplayName:   item.DisplayName,
		Name:          item.Name,
		Size:          item.Size,
		Ext:           item.Ext,
		Hash:          item.Hash,
		UUID:          item.UUID,
		MimeType:      item.MimeType,
		FileType:      item.FileType,
		IsPublic:      isPublic,
		StorageDriver: driver,
		Driver:        driver,
		Bucket:        item.Bucket,
		ObjectKey:     item.ObjectKey,
		ETag:          item.ETag,
		UploadScene:   firstNonEmpty(item.UploadScene, parent.UploadScene),
	}
}

func (s *FileResourceService) CompleteDirectUploadBatch(params *form.FileUploadCompleteBatch, uid uint) (*resources.FileUploadCompleteBatchResult, error) {
	return s.CompleteDirectUploadBatchWithContext(context.Background(), params, uid)
}

func (s *FileResourceService) CompleteDirectUploadBatchWithContext(ctx context.Context, params *form.FileUploadCompleteBatch, uid uint) (*resources.FileUploadCompleteBatchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	total := len(params.Items)
	result := &resources.FileUploadCompleteBatchResult{
		Total: int64(total),
		Items: make([]*resources.FileUploadCompleteBatchItemResult, total),
	}
	if total == 0 {
		return result, nil
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	pathCache := sync.Map{}
	getLogicalPath := func(folderID uint) (string, error) {
		if v, ok := pathCache.Load(folderID); ok {
			return v.(string), nil
		}
		path, err := folderLogicalPathTx(db, folderID)
		if err != nil {
			return "", err
		}
		pathCache.Store(folderID, path)
		return path, nil
	}
	concurrency := uploadBatchConcurrency
	if concurrency > total {
		concurrency = total
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success, failed := 0, 0
	for index, item := range params.Items {
		wg.Add(1)
		sem <- struct{}{}
		go func(index int, it form.FileUploadCompleteBatchItem) {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := s.completeDirectUploadWithPath(ctx, normalizeCompleteItem(params, it), uid, getLogicalPath)
			var itemResult *resources.FileUploadCompleteBatchItemResult
			if err != nil {
				itemResult = &resources.FileUploadCompleteBatchItemResult{
					ClientID: it.ClientID,
					Success:  false,
					Error:    buildBatchFailure(index, it.ClientID, err),
				}
			} else {
				itemResult = &resources.FileUploadCompleteBatchItemResult{
					ClientID: it.ClientID,
					Success:  true,
					Data:     data,
				}
			}
			mu.Lock()
			result.Items[index] = itemResult
			if itemResult.Success {
				success++
			} else {
				failed++
			}
			mu.Unlock()
		}(index, item)
	}
	wg.Wait()
	result.Success = int64(success)
	result.Failed = int64(failed)
	return result, nil
}

func (s *FileResourceService) CreateFromReader(ctx context.Context, input ServerGeneratedFileInput) (*model.UploadFiles, error) {
	// 自动寻回或创建统一的临时导出目录
	var expiresAtVal time.Time
	if input.FolderID == 0 {
		folderID, err := s.ensureExportTempFolder(ctx)
		if err == nil {
			input.FolderID = folderID
		} else {
			log.Logger.Warn("自动创建/寻找导出目录失败，退回到根目录", zap.Error(err))
		}
	}

	if input.ExpiresAt != nil {
		expiresAtVal = *input.ExpiresAt
	} else if input.FolderID > 0 {
		// 检查是否是默认的临时导出目录，若是则自动附加过期天数
		tempFolderID, err := s.ensureExportTempFolder(ctx)
		if err == nil && input.FolderID == tempFolderID {
			ttlDays := 7
			cfgSvc := NewStorageConfigService()
			if settings, err := cfgSvc.Get(false); err == nil && settings.Config.ExportTempFileTTL > 0 {
				ttlDays = settings.Config.ExportTempFileTTL
			}
			expiresAtVal = time.Now().AddDate(0, 0, ttlDays)
		}
	}

	driver, cfg, activeDriver, err := s.activeStorageDriver(ctx)
	if err != nil {
		return nil, err
	}
	// 通过临时文件落盘 + 边写边算 hash 的方式避免把整个 Reader 读进内存，
	// 后续 driver.Put 直接从临时文件流式上传。
	tmpFile, err := os.CreateTemp("", "upload-stream-*.tmp")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	closed := false
	defer func() {
		if !closed {
			if closeErr := tmpFile.Close(); closeErr != nil {
				log.Warn("关闭临时文件失败", zap.String("path", tmpPath), zap.Error(closeErr))
			}
		}
		if rmErr := os.Remove(tmpPath); rmErr != nil && !os.IsNotExist(rmErr) {
			log.Warn("清理临时文件失败", zap.String("path", tmpPath), zap.Error(rmErr))
		}
	}()
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(hasher, tmpFile), input.Reader)
	if err != nil {
		return nil, err
	}
	if err := tmpFile.Sync(); err != nil {
		return nil, err
	}
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	fileUUID := strings.ReplaceAll(uuid.New().String(), "-", "")
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
		OwnerUID:      input.UID,
		IsPublic:      input.IsPublic,
		Size:          uint(size),
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
			Reader:      tmpFile,
			Size:        size,
			ContentType: input.MimeType,
		})
		if err != nil {
			if delErr := db.Delete(&model.UploadFileObject{}, object.ID).Error; delErr != nil {
				log.Warn("清理 UploadFileObject 失败", zap.Uint("object_id", object.ID), zap.Error(delErr))
			}
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
	// 上传完成后立即关闭临时文件句柄，defer 仍负责 Remove。
	if closeErr := tmpFile.Close(); closeErr != nil {
		log.Warn("关闭临时文件失败", zap.String("path", tmpPath), zap.Error(closeErr))
	}
	closed = true
	logicalPath, err := s.folderLogicalPath(input.FolderID)
	if err != nil {
		return nil, err
	}

	uploadFile, existing, err := s.createUploadFileWithConflict(db, input.FolderID, input.OriginName, hash, input.UID, input.IsPublic, func(resolved *uploadFileConflictResult) *model.UploadFiles {
		originName := resolved.FinalName
		uf := &model.UploadFiles{
			UID:           input.UID,
			FolderID:      input.FolderID,
			LogicalPath:   logicalPath,
			DisplayName:   firstNonEmpty(input.DisplayName, originName),
			OriginName:    originName,
			Name:          fileUUID + ext,
			Path:          object.ObjectKey,
			Size:          uint(size),
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
			ExpiresAt:     dateutils.FormatDate{Time: expiresAtVal},
		}
		applyObjectToUploadFile(uf, object)
		return uf
	})
	if err != nil {
		return nil, err
	}
	if existing {
		// 命中已存在记录，本次新建的物理对象成为孤儿，需清理（复用对象除外）
		if !reused {
			if delErr := db.Delete(&model.UploadFileObject{}, object.ID).Error; delErr != nil {
				log.Warn("清理孤儿 UploadFileObject 失败", zap.Uint("object_id", object.ID), zap.Error(delErr))
			}
		}
		s.fillObjectReuseCounts([]*model.UploadFiles{uploadFile})
		return uploadFile, nil
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
	ExpiresAt   *time.Time
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
	mimeType := normalizeUploadMimeType(file.Header.Get("Content-Type"), filepath.Ext(file.Filename), true)
	if err := s.validateFileLimit(file.Filename, file.Size, mimeType); err != nil {
		return nil, err
	}
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
	result.MimeType = normalizeUploadMimeType(result.MimeType, firstNonEmpty(result.Ext, filepath.Ext(file.Filename)), true)
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
		OwnerUID:      uid,
		IsPublic:      params.IsPublic,
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

	uploadFile, existing, err := s.createUploadFileWithConflict(db, params.FolderID, result.OriginName, result.Sha256, uid, params.IsPublic, func(resolved *uploadFileConflictResult) *model.UploadFiles {
		originName := resolved.FinalName
		uf := &model.UploadFiles{
			UID:           uid,
			FolderID:      params.FolderID,
			LogicalPath:   logicalPath,
			DisplayName:   originName,
			OriginName:    originName,
			Name:          result.Name,
			Path:          object.ObjectKey,
			Size:          uint(result.Size),
			Ext:           firstNonEmpty(result.Ext, filepath.Ext(originName)),
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
		applyObjectToUploadFile(uf, object)
		return uf
	})
	if err != nil {
		if !reused {
			cleanupStoredUpload(result.Path)
			if delErr := db.Delete(&model.UploadFileObject{}, object.ID).Error; delErr != nil {
				log.Warn("清理 UploadFileObject 失败", zap.Uint("object_id", object.ID), zap.Error(delErr))
			}
		}
		return nil, err
	}
	if existing {
		// 命中已存在记录，本次落盘的物理文件与新建对象成为孤儿，需清理（复用对象除外）
		if !reused {
			cleanupStoredUpload(result.Path)
			if delErr := db.Delete(&model.UploadFileObject{}, object.ID).Error; delErr != nil {
				log.Warn("清理孤儿 UploadFileObject 失败", zap.Uint("object_id", object.ID), zap.Error(delErr))
			}
		}
		s.fillObjectReuseCounts([]*model.UploadFiles{uploadFile})
		return resources.NewFileResourceTransformer().ToStruct(uploadFile), nil
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
		return "", pkgErrors.NewBusinessError(pkgErrors.FolderNameInvalid)
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
		return pkgErrors.NewBusinessError(pkgErrors.FolderNameExists)
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
	if err := tx.Where("logical_path LIKE ? ESCAPE '!'", escapeLikePattern(oldPath)+"/%").Find(&children).Error; err != nil {
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
	return "uploads/" + time.Now().Format("20060102") + "/" + strings.ReplaceAll(uuid.New().String(), "-", "") + ext
}

// MultipartInit 分片上传初始化。
func (s *FileResourceService) MultipartInit(params *form.FileMultipartInit, uidArg ...uint) (*resources.MultipartInitResource, error) {
	uid := firstUint(uidArg...)
	if err := s.validateFileLimit(params.OriginName, params.Size, params.MimeType); err != nil {
		return nil, err
	}
	_, cfg, activeDriver, err := s.activeStorageDriver(context.Background())
	if err != nil {
		return nil, err
	}
	if params.Driver != "" {
		activeDriver = params.Driver
	}
	if activeDriver == model.StorageDriverLocal {
		return nil, pkgErrors.NewBusinessError(pkgErrors.LocalStorageMultipartUnsupported)
	}
	bucket := bucketForDriver(activeDriver, cfg, params.IsPublic)
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(cfg.SignedURLTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if object, err := findReusableFileObject(db, activeDriver, bucket, params.Hash, uid, params.IsPublic); err == nil {
		completeToken, _, err := createUploadSession(db, uploadSessionInput{
			UploadType:    model.UploadSessionTypeMultipart,
			StorageDriver: activeDriver,
			StorageBase:   object.StorageBase,
			Bucket:        object.Bucket,
			StoragePath:   object.StoragePath,
			ObjectKey:     object.ObjectKey,
			FileObjectID:  object.ID,
			OwnerUID:      uid,
			FolderID:      params.FolderID,
			OriginName:    params.OriginName,
			Size:          object.Size,
			Hash:          object.Hash,
			MimeType:      object.MimeType,
			ETag:          object.ETag,
			IsPublic:      params.IsPublic,
			ExpiresAt:     time.Now().Add(ttl),
		})
		if err != nil {
			return nil, err
		}
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
			CompleteToken:   completeToken,
			CompletePayload: buildMultipartCompletePayload(completeToken),
		}, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	driver, _, err := s.storageDriverByName(context.Background(), activeDriver)
	if err != nil {
		return nil, err
	}
	mp, ok := driver.(filestorage.MultipartCapable)
	if !ok {
		return nil, pkgErrors.NewBusinessError(pkgErrors.LocalStorageMultipartUnsupported)
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
	initResult, err := mp.InitMultipartUpload(context.Background(), filestorage.MultipartUploadInput{
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
		partResult, err := mp.PresignMultipartPart(context.Background(), filestorage.MultipartPartInput{
			Bucket:     bucket,
			ObjectKey:  objectKey,
			UploadID:   initResult.UploadID,
			PartNumber: int32(i),
		})
		if err != nil {
			if abortErr := mp.AbortMultipartUpload(context.Background(), bucket, objectKey, initResult.UploadID); abortErr != nil {
				log.Warn("中止分片上传失败",
					zap.String("bucket", bucket), zap.String("object_key", objectKey),
					zap.String("upload_id", initResult.UploadID), zap.Error(abortErr))
			}
			return nil, err
		}
		parts = append(parts, &resources.MultipartPartResource{
			PartNumber: partResult.PartNumber,
			UploadURL:  partResult.UploadURL,
			Headers:    partResult.Headers,
		})
	}
	completeToken, _, err := createUploadSession(db, uploadSessionInput{
		UploadType:    model.UploadSessionTypeMultipart,
		StorageDriver: activeDriver,
		Bucket:        initResult.Bucket,
		ObjectKey:     initResult.ObjectKey,
		UploadID:      initResult.UploadID,
		OwnerUID:      uid,
		FolderID:      params.FolderID,
		OriginName:    params.OriginName,
		Size:          uint(params.Size),
		Hash:          params.Hash,
		MimeType:      params.MimeType,
		IsPublic:      params.IsPublic,
		ExpiresAt:     time.Now().Add(ttl),
	})
	if err != nil {
		if abortErr := mp.AbortMultipartUpload(context.Background(), initResult.Bucket, initResult.ObjectKey, initResult.UploadID); abortErr != nil {
			log.Warn("创建分片上传 session 失败后中止上传失败",
				zap.String("bucket", initResult.Bucket), zap.String("object_key", initResult.ObjectKey),
				zap.String("upload_id", initResult.UploadID), zap.Error(abortErr))
		}
		return nil, err
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
		CompleteToken:   completeToken,
		CompletePayload: buildMultipartCompletePayload(completeToken),
	}, nil
}

// MultipartComplete 分片上传完成。
func (s *FileResourceService) MultipartComplete(params *form.FileMultipartComplete, uid uint) (*resources.FileResourceResources, error) {
	return s.MultipartCompleteWithContext(context.Background(), params, uid)
}

func (s *FileResourceService) MultipartCompleteWithContext(ctx context.Context, params *form.FileMultipartComplete, uid uint) (*resources.FileResourceResources, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	sniffTarget, err := s.prepareMultipartCompleteObject(ctx, params, uid)
	if err != nil {
		return nil, err
	}
	if err := s.validateUploadObjectMime(ctx, "multipart upload", sniffTarget.storageDriver, sniffTarget.bucket, sniffTarget.objectKey, sniffTarget.mimeType); err != nil {
		restoreUploadSessionPending(context.Background(), db, sniffTarget.sessionID)
		return nil, err
	}
	// 事务外校验实际文件大小：防止攻击者伪造小 size 上传超限文件
	if err := s.validateActualObjectSize(ctx, sniffTarget.storageDriver, sniffTarget.bucket, sniffTarget.objectKey, sniffTarget.declaredSize); err != nil {
		restoreUploadSessionPending(context.Background(), db, sniffTarget.sessionID)
		return nil, err
	}
	var uploadFile *model.UploadFiles
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		session, err := findCompletingUploadSession(tx, sniffTarget.sessionID, uid, model.UploadSessionTypeMultipart)
		if err != nil {
			return err
		}
		if err := validateMultipartCompleteSession(session, params, uid); err != nil {
			return err
		}
		if err := requireUploadSessionObject(session); err != nil {
			return err
		}
		storageDriver := session.StorageDriver
		var object *model.UploadFileObject
		var reusedObject bool
		if session.FileObjectID > 0 {
			object, err = findFileObjectByID(tx, session.FileObjectID)
			if err != nil {
				return err
			}
			if err := validateFileObjectMatchesSession(object, session); err != nil {
				return err
			}
		} else {
			object, reusedObject, err = ensureFileObject(tx, uploadFileObjectInput{
				StorageDriver: storageDriver,
				StorageBase:   storageBaseForDriver(storageDriver, session.IsPublic == global.Yes, session.Bucket),
				Bucket:        session.Bucket,
				StoragePath:   session.ObjectKey,
				ObjectKey:     session.ObjectKey,
				OwnerUID:      uid,
				IsPublic:      session.IsPublic,
				Size:          session.Size,
				Hash:          session.Hash,
				MimeType:      session.MimeType,
				ETag:          "",
				Status:        model.StorageStatusStored,
			})
			if err != nil {
				return err
			}
		}
		logicalPath, err := folderLogicalPathTx(tx, session.FolderID)
		if err != nil {
			return err
		}
		bucket := object.Bucket
		objectKey := object.ObjectKey
		size := session.Size
		hash := session.Hash
		mimeType := session.MimeType
		fileUUID := strings.ReplaceAll(uuid.New().String(), "-", "")

		fileType, err := resolveUploadFileType(params.FileType, mimeType)
		if err != nil {
			return err
		}

		created, existing, err := s.createUploadFileWithConflict(tx, session.FolderID, session.OriginName, hash, uid, session.IsPublic, func(resolved *uploadFileConflictResult) *model.UploadFiles {
			originName := resolved.FinalName
			uf := &model.UploadFiles{
				UID:           uid,
				FolderID:      session.FolderID,
				LogicalPath:   logicalPath,
				DisplayName:   originName,
				OriginName:    originName,
				Name:          filepath.Base(objectKey),
				Path:          objectKey,
				Size:          size,
				Ext:           filepath.Ext(originName),
				Hash:          hash,
				UUID:          fileUUID,
				MimeType:      mimeType,
				FileType:      fileType,
				IsPublic:      session.IsPublic,
				StorageDriver: storageDriver,
				StorageBase:   storageBaseForDriver(storageDriver, session.IsPublic == global.Yes, bucket),
				Bucket:        bucket,
				StoragePath:   objectKey,
				ObjectKey:     objectKey,
				ETag:          "",
				StorageStatus: model.StorageStatusStored,
				UploadSource:  model.UploadSourceDirect,
				UploadScene:   session.UploadScene,
				UploadStatus:  model.UploadStatusUploaded,
			}
			applyObjectToUploadFile(uf, object)
			return uf
		})
		if err != nil {
			return err
		}
		if err := markUploadSessionCompleted(tx, session, created.ID, object.ID); err != nil {
			return err
		}
		if existing && session.FileObjectID == 0 && !reusedObject {
			if delErr := tx.Delete(&model.UploadFileObject{}, object.ID).Error; delErr != nil {
				log.Warn("multipart orphan cleanup: delete UploadFileObject failed",
					zap.Uint("object_id", object.ID), zap.Error(delErr))
			}
			driver, _, drvErr := s.storageDriverByName(context.Background(), storageDriver)
			if drvErr == nil {
				if delErr := driver.Delete(context.Background(), session.Bucket, session.ObjectKey); delErr != nil {
					log.Warn("multipart orphan cleanup: delete object failed",
						zap.String("bucket", session.Bucket), zap.String("object_key", session.ObjectKey), zap.Error(delErr))
				}
			}
		}
		uploadFile = created
		return nil
	})
	if err != nil {
		restoreUploadSessionPending(context.Background(), db, sniffTarget.sessionID)
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

func buildMultipartCompletePayload(completeToken string) map[string]any {
	return map[string]any{
		"complete_token": completeToken,
	}
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
	mp, ok := driver.(filestorage.MultipartCapable)
	if !ok {
		return fmt.Errorf("storage driver %q does not support multipart upload", storageDriver)
	}
	return mp.AbortMultipartUpload(context.Background(), params.Bucket, params.ObjectKey, params.UploadID)
}

// validateFileLimit 验证文件大小与 MIME 类型限制。
func (s *FileResourceService) validateFileLimit(fileName string, size int64, mimeType string) error {
	_, cfg, _, err := s.activeStorageDriver(context.Background())
	if err != nil {
		return err
	}

	maxSizeMB := cfg.MaxFileSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = 10
	}
	maxSizeBytes := int64(maxSizeMB) * 1024 * 1024
	if size > maxSizeBytes {
		return pkgErrors.NewBusinessError(pkgErrors.FileTooLarge)
	}

	if len(cfg.AllowedExtensions) > 0 {
		allowed := false
		fileExt := strings.ToLower(filepath.Ext(fileName))
		matchedPattern := ""
		for _, extPattern := range cfg.AllowedExtensions {
			extPattern = strings.ToLower(strings.TrimSpace(extPattern))
			if extPattern == "" {
				continue
			}
			if extPattern == "*" {
				allowed = true
				matchedPattern = "*"
				break
			}
			if !strings.HasPrefix(extPattern, ".") {
				extPattern = "." + extPattern
			}
			if fileExt == extPattern {
				allowed = true
				matchedPattern = extPattern
				break
			}
		}
		if !allowed {
			return pkgErrors.NewBusinessError(pkgErrors.FileMimeTypeNotAllowed)
		}

		// 后缀校验通过后，如果匹配的不是通配符 "*"，自动推导标准 MIME 并强校验上传的 MIME 是否兼容
		if matchedPattern != "*" && matchedPattern != "" {
			if expectedMime := mime.TypeByExtension(matchedPattern); expectedMime != "" {
				expectedMime = strings.ToLower(strings.TrimSpace(strings.Split(expectedMime, ";")[0]))

				cleanMime := normalizeUploadMimeType(mimeType, fileExt, true)

				if (cleanMime == "" || cleanMime == "application/octet-stream") && fileName != "" {
					if guessedMime := mime.TypeByExtension(fileExt); guessedMime != "" {
						cleanMime = normalizeUploadMimeType(guessedMime, "", false)
					}
				}

				if cleanMime == "" {
					cleanMime = "application/octet-stream"
				}

				if !isMimeCompatible(cleanMime, expectedMime) {
					return pkgErrors.NewBusinessError(pkgErrors.FileMimeTypeNotAllowed)
				}
			}
		}
	}

	if len(cfg.AllowedMimeTypes) > 0 {
		allowed := false
		cleanMime := normalizeUploadMimeType(mimeType, filepath.Ext(fileName), true)

		if (cleanMime == "" || cleanMime == "application/octet-stream") && fileName != "" {
			ext := filepath.Ext(fileName)
			if ext != "" {
				cleanMime = normalizeUploadMimeType(mime.TypeByExtension(ext), "", false)
			}
		}

		if cleanMime == "" {
			cleanMime = "application/octet-stream"
		}

		for _, pattern := range cfg.AllowedMimeTypes {
			pattern = strings.ToLower(strings.TrimSpace(pattern))
			if pattern == "" {
				continue
			}
			if pattern == "*" || pattern == "*/*" {
				allowed = true
				break
			}
			if strings.Contains(pattern, "*") {
				prefix := strings.ReplaceAll(pattern, "*", "")
				if strings.HasPrefix(cleanMime, prefix) {
					allowed = true
					break
				}
			} else {
				if cleanMime == pattern {
					allowed = true
					break
				}
			}
		}

		if !allowed {
			return pkgErrors.NewBusinessError(pkgErrors.FileMimeTypeNotAllowed)
		}
	}

	return nil
}

func normalizeUploadMimeType(mimeType, ext string, inferFromExt bool) string {
	cleanMime := strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(cleanMime, ";"); idx >= 0 {
		cleanMime = strings.TrimSpace(cleanMime[:idx])
	}
	if inferFromExt && (cleanMime == "" || cleanMime == "application/octet-stream") {
		if guessedMime := mime.TypeByExtension(ext); guessedMime != "" {
			return normalizeUploadMimeType(guessedMime, "", false)
		}
	}
	return cleanMime
}

func isMimeCompatible(actual, expected string) bool {
	actual = strings.ToLower(strings.TrimSpace(actual))
	expected = strings.ToLower(strings.TrimSpace(expected))
	if actual == "text/plain" && expected == "text/csv" {
		// CSV 没有专用文件头，标准库会将其识别为纯文本。
		return true
	}
	if actual == expected {
		return true
	}
	if actual == "application/octet-stream" {
		return true
	}

	groups := [][]string{
		{"image/jpeg", "image/jpg", "image/pjpeg"},
		{"image/png", "image/x-png"},
		{"text/javascript", "application/javascript", "application/x-javascript"},
		{"text/xml", "application/xml"},
	}

	for _, g := range groups {
		hasActual := false
		hasExpected := false
		for _, m := range g {
			if m == actual {
				hasActual = true
			}
			if m == expected {
				hasExpected = true
			}
		}
		if hasActual && hasExpected {
			return true
		}
	}
	return false
}

// mimeSniffHeaderSize 读取前 512 字节用于 MIME 类型嗅探。
const mimeSniffHeaderSize = 512

type uploadMimeSniffTarget struct {
	sessionID     uint
	storageDriver string
	bucket        string
	objectKey     string
	mimeType      string
	declaredSize  uint
}

// validateActualObjectSize 从存储驱动读取对象的实际大小，校验：
// 1. 实际大小不超过配置上限（MaxFileSizeMB）
// 2. 实际大小与 session 中声明的大小一致
// 如果驱动不支持 Stat，则跳过校验（向后兼容）。
func (s *FileResourceService) validateActualObjectSize(ctx context.Context, storageDriver, bucket, objectKey string, declaredSize uint) error {
	driver, cfg, err := s.storageDriverByName(ctx, storageDriver)
	if err != nil {
		return fmt.Errorf("resolve storage driver for size validation: %w", err)
	}
	sc, ok := driver.(filestorage.StatCapable)
	if !ok {
		return nil // 驱动不支持 Stat，跳过校验
	}
	stat, err := sc.Stat(ctx, bucket, objectKey)
	if err != nil {
		return fmt.Errorf("stat object for size validation: %w", err)
	}
	// 校验实际大小不超过配置上限
	maxSizeMB := cfg.MaxFileSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = 10
	}
	maxSizeBytes := int64(maxSizeMB) * 1024 * 1024
	if stat.Size > maxSizeBytes {
		return pkgErrors.NewBusinessError(pkgErrors.FileTooLarge)
	}
	// 校验实际大小与声明大小一致
	if declaredSize > 0 && stat.Size != int64(declaredSize) {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "actual file size does not match declared size")
	}
	return nil
}

func (s *FileResourceService) resolveDirectUploadMimeSniffTarget(ctx context.Context, params *form.FileUploadComplete, uid uint) (uploadMimeSniffTarget, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return uploadMimeSniffTarget{}, err
	}
	session, err := findPendingUploadSession(db.WithContext(ctx), params.CompleteToken, model.UploadSessionTypeDirect, uid)
	if err != nil {
		return uploadMimeSniffTarget{}, err
	}
	if err := validateDirectCompleteSession(session, params, uid); err != nil {
		return uploadMimeSniffTarget{}, err
	}
	if err := requireUploadSessionObject(session); err != nil {
		return uploadMimeSniffTarget{}, err
	}
	target := uploadMimeSniffTarget{
		storageDriver: session.StorageDriver,
		bucket:        session.Bucket,
		objectKey:     session.ObjectKey,
		mimeType:      session.MimeType,
		declaredSize:  session.Size,
	}
	if session.FileObjectID > 0 {
		object, err := findFileObjectByID(db.WithContext(ctx), session.FileObjectID)
		if err != nil {
			return uploadMimeSniffTarget{}, err
		}
		if err := validateFileObjectMatchesSession(object, session); err != nil {
			return uploadMimeSniffTarget{}, err
		}
		target.bucket = object.Bucket
		target.objectKey = object.ObjectKey
	}
	return target, nil
}

func (s *FileResourceService) prepareMultipartCompleteObject(ctx context.Context, params *form.FileMultipartComplete, uid uint) (target uploadMimeSniffTarget, err error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return uploadMimeSniffTarget{}, err
	}
	var session *model.UploadSession
	if err := access.RunInTransaction(db.WithContext(ctx), func(tx *gorm.DB) error {
		var findErr error
		session, findErr = findPendingUploadSession(tx, params.CompleteToken, model.UploadSessionTypeMultipart, uid)
		if findErr != nil {
			return findErr
		}
		if err := validateMultipartCompleteSession(session, params, uid); err != nil {
			return err
		}
		if err := requireUploadSessionObject(session); err != nil {
			return err
		}
		return claimUploadSessionCompleting(tx, session)
	}); err != nil {
		return uploadMimeSniffTarget{}, err
	}
	claimedSessionID := session.ID
	defer func() {
		if err != nil && claimedSessionID > 0 {
			restoreUploadSessionPending(context.Background(), db, claimedSessionID)
		}
	}()
	target = uploadMimeSniffTarget{
		sessionID:     session.ID,
		storageDriver: session.StorageDriver,
		bucket:        session.Bucket,
		objectKey:     session.ObjectKey,
		mimeType:      session.MimeType,
		declaredSize:  session.Size,
	}
	if session.FileObjectID > 0 {
		object, err := findFileObjectByID(db.WithContext(ctx), session.FileObjectID)
		if err != nil {
			return uploadMimeSniffTarget{}, err
		}
		if err := validateFileObjectMatchesSession(object, session); err != nil {
			return uploadMimeSniffTarget{}, err
		}
		target.bucket = object.Bucket
		target.objectKey = object.ObjectKey
		return target, nil
	}
	if strings.TrimSpace(session.UploadID) == "" {
		return uploadMimeSniffTarget{}, fmt.Errorf("upload_id is required")
	}
	completeParts, err := validateMultipartCompleteParts(params.Parts)
	if err != nil {
		return uploadMimeSniffTarget{}, err
	}
	driver, _, err := s.storageDriverByName(ctx, session.StorageDriver)
	if err != nil {
		return uploadMimeSniffTarget{}, err
	}
	mp, ok := driver.(filestorage.MultipartCapable)
	if !ok {
		return uploadMimeSniffTarget{}, pkgErrors.NewBusinessError(pkgErrors.LocalStorageMultipartUnsupported)
	}
	if err := mp.CompleteMultipartUpload(ctx, session.Bucket, session.ObjectKey, session.UploadID, completeParts); err != nil {
		return uploadMimeSniffTarget{}, err
	}
	return target, nil
}

func (s *FileResourceService) validateUploadObjectMime(ctx context.Context, operation, storageDriver, bucket, objectKey, mimeType string) error {
	sniffedMIME, valid, sniffErr := s.sniffAndValidateMimeType(ctx, storageDriver, bucket, objectKey, mimeType)
	if sniffErr != nil {
		log.Warn(operation+" MIME sniff failed",
			zap.String("bucket", bucket), zap.String("object_key", objectKey), zap.Error(sniffErr))
		return pkgErrors.NewBusinessError(pkgErrors.FileMimeTypeNotAllowed)
	}
	if !valid {
		log.Warn(operation+" MIME mismatch: declared vs detected",
			zap.String("declared", mimeType), zap.String("detected", sniffedMIME),
			zap.String("object_key", objectKey))
		return pkgErrors.NewBusinessError(pkgErrors.FileMimeTypeNotAllowed)
	}
	return nil
}

// sniffAndValidateMimeType 从已上传对象中读取前 512 字节，使用 http.DetectContentType 嗅探真实 MIME 类型，
// 与声明的 declaredMIME 进行宽松比较。返回值：
//   - sniffedMIME: 嗅探到的 MIME 类型（失败时返回空串）
//   - valid: 声明 MIME 与嗅探结果是否兼容
//   - err: 非 nil 表示读取对象失败（调用方应拒绝完成上传）
func (s *FileResourceService) sniffAndValidateMimeType(ctx context.Context, storageDriver, bucket, objectKey, declaredMIME string) (sniffedMIME string, valid bool, err error) {
	driver, _, err := s.storageDriverByName(ctx, storageDriver)
	if err != nil {
		return "", false, fmt.Errorf("resolve storage driver for MIME sniff: %w", err)
	}
	reader, err := driver.Open(ctx, bucket, objectKey)
	if err != nil {
		return "", false, fmt.Errorf("open object for MIME sniff: %w", err)
	}
	defer reader.Close()
	buf := make([]byte, mimeSniffHeaderSize)
	// io.ReadFull 返回 io.EOF 或 io.ErrUnexpectedEOF 表示文件不足 512 字节，这不算错误。
	n, readErr := io.ReadFull(reader, buf)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return "", false, fmt.Errorf("read object for MIME sniff: %w", readErr)
	}
	if n == 0 {
		return "", false, fmt.Errorf("empty object for MIME sniff")
	}
	buf = buf[:n]
	sniffedMIME = http.DetectContentType(buf)

	// 有些检测结果会带 charset 参数，只取 MIME 部分
	if idx := strings.Index(sniffedMIME, ";"); idx >= 0 {
		sniffedMIME = strings.TrimSpace(sniffedMIME[:idx])
	}

	if declaredMIME == "" {
		return sniffedMIME, true, nil
	}
	valid = isMimeCompatible(sniffedMIME, declaredMIME)
	return sniffedMIME, valid, nil
}

// SystemTempFolderName 系统临时文件夹常量，对前端隐藏。
const SystemTempFolderName = "system_temp"

// SystemAvatarsFolderName 系统头像文件夹常量，对前端隐藏。
const SystemAvatarsFolderName = "system_avatars"

func (s *FileResourceService) ensureExportTempFolder(ctx context.Context) (uint, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return 0, err
	}

	var folder model.UploadFileFolder
	err = db.Where("parent_id = 0 AND name = ?", SystemTempFolderName).First(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			folder = model.UploadFileFolder{
				ParentID:    0,
				Name:        SystemTempFolderName,
				LogicalPath: "/" + SystemTempFolderName,
				IsHidden:    1,
				CreatedBy:   0,
				UpdatedBy:   0,
			}
			if err := db.Create(&folder).Error; err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}

	// 自愈：历史上可能存在同名但 is_hidden=0 的目录
	if folder.IsHidden == 0 {
		if err := db.Model(&model.UploadFileFolder{}).Where("id = ?", folder.ID).Update("is_hidden", 1).Error; err != nil {
			return 0, err
		}
	}

	return folder.ID, nil
}

func (s *FileResourceService) EnsureAvatarFolder(ctx context.Context) (uint, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return 0, err
	}

	var folder model.UploadFileFolder
	err = db.Where("parent_id = 0 AND name = ?", SystemAvatarsFolderName).First(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			folder = model.UploadFileFolder{
				ParentID:    0,
				Name:        SystemAvatarsFolderName,
				LogicalPath: "/" + SystemAvatarsFolderName,
				IsHidden:    1,
				CreatedBy:   0,
				UpdatedBy:   0,
			}
			if err := db.Create(&folder).Error; err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}

	// 自愈：历史上可能存在同名但 is_hidden=0 的目录
	if folder.IsHidden == 0 {
		if err := db.Model(&model.UploadFileFolder{}).Where("id = ?", folder.ID).Update("is_hidden", 1).Error; err != nil {
			return 0, err
		}
	}

	return folder.ID, nil
}

const maxAutoRenameRetries = 100

// uploadFileConflictResult 描述同名文件冲突检查的结果。
// Existing 非 nil 时表示应跳过（同目录同名同 hash），直接返回已存在记录。
// Existing 为 nil 时，FinalName 为最终应使用的文件名（可能自动加了序号后缀）。
type uploadFileConflictResult struct {
	Existing  *model.UploadFiles // 非 nil 表示应跳过，指向已存在的记录
	FinalName string             // 最终应使用的文件名
}

// resolveUploadFileConflict 检查同目录下同名文件冲突。
//
// 规则：
//  1. 同 folder_id + 同 origin_name + 同 hash + 同上传者 + 同公开状态 → 跳过，返回已存在记录
//  2. 同 folder_id + 同 origin_name，但内容或归属/公开状态不同 → 自动加序号后缀 (1), (2), ...
//  3. 无冲突 → 返回原名称
func (s *FileResourceService) resolveUploadFileConflict(tx *gorm.DB, folderID uint, originName string, hash string, uid uint, isPublic uint8) (*uploadFileConflictResult, error) {
	// 同目录、同名：单条查询替代两次顺序 SELECT，在 Go 层按 hash 判断走复用还是重命名。
	var existing model.UploadFiles
	err := tx.Where("folder_id = ? AND origin_name = ?", folderID, originName).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &uploadFileConflictResult{FinalName: originName}, nil
	}
	if err != nil {
		return nil, err
	}
	if existing.Hash == hash && existing.UID == uid && existing.IsPublic == isPublic {
		// 逻辑记录只复用相同上传者及公开状态；物理对象可按自己的规则去重。
		return &uploadFileConflictResult{Existing: &existing, FinalName: originName}, nil
	}
	// 内容或归属/公开状态不同，保留独立逻辑记录并自动重命名。

	// 自动加序号后缀：一次 LIKE 批量查询已有重命名，内存去重替代 N 次 DB 查询
	ext := filepath.Ext(originName)
	base := strings.TrimSuffix(originName, ext)
	// 转义 LIKE 通配符，使用 ! 避免依赖数据库的反斜杠转义模式。
	likePattern := escapeLikePattern(base) + " (%)" + escapeLikePattern(ext)
	var existingNames []string
	if err := tx.Model(&model.UploadFiles{}).
		Where("folder_id = ? AND origin_name LIKE ? ESCAPE '!'", folderID, likePattern).
		Pluck("origin_name", &existingNames).Error; err != nil {
		return nil, err
	}
	used := make(map[string]bool, len(existingNames))
	for _, name := range existingNames {
		used[name] = true
	}
	for i := 1; i <= maxAutoRenameRetries; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if !used[candidate] {
			return &uploadFileConflictResult{FinalName: candidate}, nil
		}
	}
	return nil, fmt.Errorf("unable to resolve name conflict for '%s' after %d attempts", originName, maxAutoRenameRetries)
}

// escapeLikePattern 转义 SQL LIKE 通配符及 !，配合 ESCAPE '!' 使用。
func escapeLikePattern(s string) string {
	replacer := strings.NewReplacer(`!`, `!!`, `%`, `!%`, `_`, `!_`)
	return replacer.Replace(s)
}

// maxUploadConflictRetries 并发插入命中唯一索引时的重试次数。
// 每次重试都会重新解析冲突（可能转为复用或重命名），覆盖并发窗口内被其他请求抢先插入的场景。
const maxUploadConflictRetries = 5

// uploadFileConflictBuilder 根据冲突解析结果构造待插入记录。
// resolved.Existing 非 nil 时跳过插入直接复用；否则用 resolved.FinalName 作为最终文件名。
type uploadFileConflictBuilder func(resolved *uploadFileConflictResult) *model.UploadFiles

// createUploadFileWithConflict 在事务内原子地完成「解析同名冲突 → 插入记录」，
// 由 DB 唯一索引 uniq_folder_name_deleted 兜底并发竞态：检查与插入间被其他请求抢先插入时，
// Create 触发唯一冲突，重新进入下一轮解析（转为复用或继续重命名），消除 TOCTOU。
//
// 返回值 existing 为 true 表示命中已存在记录（复用，未新建）；此时调用方需自行清理本次新建的物理对象/文件。
func (s *FileResourceService) createUploadFileWithConflict(
	db *gorm.DB,
	folderID uint,
	originName string,
	hash string,
	uid uint,
	isPublic uint8,
	build uploadFileConflictBuilder,
) (file *model.UploadFiles, existing bool, err error) {
	for attempt := 0; attempt < maxUploadConflictRetries; attempt++ {
		txErr := access.RunInTransaction(db, func(tx *gorm.DB) error {
			resolved, rErr := s.resolveUploadFileConflict(tx, folderID, originName, hash, uid, isPublic)
			if rErr != nil {
				return rErr
			}
			if resolved.Existing != nil {
				file = resolved.Existing
				existing = true
				return nil
			}
			existing = false
			uploadFile := build(resolved)
			if cErr := tx.Create(uploadFile).Error; cErr != nil {
				return cErr
			}
			file = uploadFile
			return nil
		})
		if txErr == nil {
			return file, existing, nil
		}
		if isUploadNameDuplicateErr(txErr) {
			continue // 并发抢先插入，重新解析冲突
		}
		return nil, false, txErr
	}
	return nil, false, fmt.Errorf("unable to create upload file for '%s' after %d attempts due to name conflict", originName, maxUploadConflictRetries)
}

// isUploadNameDuplicateErr 识别 upload_files 同目录同名唯一索引冲突（MySQL 1062 / SQLite UNIQUE constraint）。
func isUploadNameDuplicateErr(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062 && strings.Contains(mysqlErr.Message, "uniq_folder_name_deleted")
	}
	msg := err.Error()
	if strings.Contains(msg, "Duplicate entry") && strings.Contains(msg, "uniq_folder_name_deleted") {
		return true
	}
	// SQLite：UNIQUE constraint failed: upload_files.folder_id, upload_files.origin_name, upload_files.deleted_at
	return strings.Contains(msg, "UNIQUE constraint failed") && strings.Contains(msg, "upload_files.origin_name")
}
