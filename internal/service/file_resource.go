package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

// FileResourceService 文件资源管理服务。
type FileResourceService struct {
	Base
	db                    *gorm.DB
	storageDriverResolver func(context.Context, string) (filestorage.Driver, filestorage.Config, error)
	activeStorageResolver func(context.Context) (filestorage.Driver, filestorage.Config, string, error)
}

// FileReferencedDeleteError 表示文件仍被业务数据引用，删除需要返回引用来源给前端。
type FileReferencedDeleteError struct {
	businessErr *e.BusinessError
	References  []*resources.FileReferenceResources
}

func NewFileReferencedDeleteError(references []*resources.FileReferenceResources) *FileReferencedDeleteError {
	return &FileReferencedDeleteError{
		businessErr: e.NewBusinessError(e.FileReferenced),
		References:  references,
	}
}

func (err *FileReferencedDeleteError) Error() string {
	if err == nil || err.businessErr == nil {
		return ""
	}
	return err.businessErr.Error()
}

func (err *FileReferencedDeleteError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.businessErr
}

func (err *FileReferencedDeleteError) BusinessError() *e.BusinessError {
	if err == nil {
		return nil
	}
	return err.businessErr
}

// FileResourceServiceDeps 描述 FileResourceService 可注入依赖。
type FileResourceServiceDeps struct {
	DB                    *gorm.DB
	StorageDriverResolver func(context.Context, string) (filestorage.Driver, filestorage.Config, error)
	ActiveStorageResolver func(context.Context) (filestorage.Driver, filestorage.Config, string, error)
}

func NewFileResourceService() *FileResourceService {
	return NewFileResourceServiceWithDeps(FileResourceServiceDeps{})
}

func NewFileResourceServiceWithDeps(deps FileResourceServiceDeps) *FileResourceService {
	return &FileResourceService{db: deps.DB, storageDriverResolver: deps.StorageDriverResolver, activeStorageResolver: deps.ActiveStorageResolver}
}

// List 分页查询文件资源列表。
func (s *FileResourceService) List(params *form.FileResourceList) *resources.Collection {
	query := query_builder.New().
		AddLike("origin_name", params.OriginName).
		AddLike("uuid", params.UUID).
		AddLike("mime_type", params.MimeType).
		AddEq("file_type", params.FileType).
		AddEq("is_public", params.IsPublic).
		AddEq("storage_driver", params.StorageDriver).
		AddEq("storage_status", params.StorageStatus)
	if params.UID > 0 {
		query.AddEq("uid", params.UID)
	}
	if params.FolderID != nil {
		if params.IncludeSubfolder == global.Yes {
			folderIDs := s.descendantFolderIDs(*params.FolderID)
			folderIDs = append(folderIDs, *params.FolderID)
			query.AddCondition("folder_id IN ?", folderIDs)
		} else {
			query.AddEq("folder_id", *params.FolderID)
		}
	}
	if params.IsReferenced != nil {
		if *params.IsReferenced == global.Yes {
			query.AddCondition("EXISTS (SELECT 1 FROM upload_file_references WHERE upload_file_references.file_id = upload_files.id)")
		} else {
			query.AddCondition("NOT EXISTS (SELECT 1 FROM upload_file_references WHERE upload_file_references.file_id = upload_files.id)")
		}
	}
	if params.StartTime != "" {
		query.AddCondition("created_at >= ?", params.StartTime)
	}
	if params.EndTime != "" {
		query.AddCondition("created_at <= ?", params.EndTime)
	}
	condition, args := query.Build()

	transformer := resources.NewFileResourceTransformer()
	total, collection, err := s.listUploadFiles(params, condition, args)
	if err != nil {
		log.Logger.Error("查询文件资源列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// Detail 查询文件资源详情。
func (s *FileResourceService) Detail(id uint) (any, error) {
	uploadFile, err := s.findByID(id)
	if err != nil {
		return nil, err
	}
	refs, _ := NewFileReferenceService(s.db).ReferencesByFileID(uploadFile.ID)
	uploadFile.ReferenceCount = int64(len(refs))
	s.fillObjectReuseCounts([]*model.UploadFiles{uploadFile})
	result := resources.NewFileResourceTransformer().ToStruct(uploadFile)
	if data := result; data != nil {
		items := make([]any, 0, len(refs))
		transformer := resources.NewFileReferenceTransformer()
		for _, ref := range refs {
			items = append(items, transformer.ToStruct(ref))
		}
		data.References = items
	}
	return result, nil
}

// Delete 软删除文件记录。
func (s *FileResourceService) Delete(id uint, deletedBy uint, reason string) error {
	uploadFile, err := s.findByID(id)
	if err != nil {
		return err
	}
	refs, err := NewFileReferenceService(s.db).ReferencesByFileID(id)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		return NewFileReferencedDeleteError(buildFileReferenceResources(refs))
	}

	if s.db != nil {
		uploadFile.SetDB(s.db)
	}
	_ = uploadFile.UpdateById(id, map[string]any{"deleted_by": deletedBy, "deleted_reason": reason})
	rowsAffected, err := uploadFile.DeleteByID(id)
	if err != nil {
		log.Logger.Error("删除文件资源记录失败", zap.Error(err), zap.Uint("id", id))
		return err
	}
	if rowsAffected == 0 {
		return e.NewBusinessError(e.NotFound)
	}
	return nil
}

func (s *FileResourceService) Restore(id uint) error {
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	result := db.Unscoped().Model(&model.UploadFiles{}).Where("id = ? AND deleted_at <> 0", id).Updates(map[string]any{
		"deleted_at":     0,
		"deleted_by":     0,
		"deleted_reason": "",
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return e.NewBusinessError(e.NotFound)
	}
	return nil
}

func (s *FileResourceService) Destroy(id uint) error {
	uploadFile, err := s.findByIDUnscoped(id)
	if err != nil {
		return err
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	result := db.Unscoped().Delete(&model.UploadFiles{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return e.NewBusinessError(e.NotFound)
	}
	_ = db.Where("file_id = ?", id).Delete(&model.UploadFileReference{}).Error
	if uploadFile.FileObjectID > 0 {
		return s.deleteObjectIfUnreferenced(db, uploadFile.FileObjectID)
	}
	if err := s.deletePhysicalFile(uploadFile); err != nil {
		return err
	}
	return nil
}

func (s *FileResourceService) References(params *form.FileReferenceList) *resources.Collection {
	return NewFileReferenceService(s.db).List(params)
}

func buildFileReferenceResources(refs []*model.UploadFileReference) []*resources.FileReferenceResources {
	items := make([]*resources.FileReferenceResources, 0, len(refs))
	transformer := resources.NewFileReferenceTransformer()
	for _, ref := range refs {
		items = append(items, transformer.ToStruct(ref))
	}
	return items
}

func (s *FileResourceService) findByID(id uint) (*model.UploadFiles, error) {
	uploadFile := model.NewUploadFiles()
	if s.db != nil {
		uploadFile.SetDB(s.db)
	}
	if err := uploadFile.GetById(id); err != nil || uploadFile.ID == 0 {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Logger.Error("查询文件资源失败", zap.Error(err), zap.Uint("id", id))
		}
		return nil, e.NewBusinessError(e.NotFound)
	}
	return uploadFile, nil
}

func (s *FileResourceService) findByIDUnscoped(id uint) (*model.UploadFiles, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	var uploadFile model.UploadFiles
	if err := db.Unscoped().Where("id = ?", id).First(&uploadFile).Error; err != nil || uploadFile.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return &uploadFile, nil
}

func (s *FileResourceService) deletePhysicalFile(uploadFile *model.UploadFiles) error {
	if uploadFile == nil {
		return nil
	}
	driverName := firstNonEmpty(uploadFile.StorageDriver, model.StorageDriverLocal)
	var driver filestorage.Driver
	var err error
	if driverName == model.StorageDriverLocal && uploadFile.StorageBase != "" {
		driver = filestorage.NewLocalDriver(filestorage.LocalConfig{
			PublicBasePath:  uploadFile.StorageBase,
			PrivateBasePath: uploadFile.StorageBase,
		}, uploadFile.StorageBase, uploadFile.StorageBase)
	} else {
		driver, _, err = s.storageDriverByName(context.Background(), driverName)
	}
	if err != nil {
		return err
	}
	objectKey := firstNonEmpty(uploadFile.ObjectKey, uploadFile.StoragePath, uploadFile.Path)
	if objectKey == "" {
		return nil
	}
	if err := driver.Delete(context.Background(), uploadFile.Bucket, objectKey); err != nil {
		log.Logger.Error("删除物理文件失败", zap.Error(err), zap.Uint("id", uploadFile.ID), zap.String("object_key", objectKey))
		return err
	}
	return nil
}

func (s *FileResourceService) storageDriverByName(ctx context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
	if s.storageDriverResolver != nil {
		return s.storageDriverResolver(ctx, driverName)
	}
	return NewStorageDriverByName(ctx, driverName)
}

func (s *FileResourceService) activeStorageDriver(ctx context.Context) (filestorage.Driver, filestorage.Config, string, error) {
	if s.activeStorageResolver != nil {
		return s.activeStorageResolver(ctx)
	}
	return NewActiveStorageDriver(ctx)
}

func (s *FileResourceService) descendantFolderIDs(folderID uint) []uint {
	db, err := s.dbOrDefault()
	if err != nil || folderID == 0 {
		return nil
	}
	var ids []uint
	current := []uint{folderID}
	for len(current) > 0 {
		var children []uint
		if err := db.Model(&model.UploadFileFolder{}).Where("parent_id IN ?", current).Pluck("id", &children).Error; err != nil || len(children) == 0 {
			break
		}
		ids = append(ids, children...)
		current = children
	}
	return ids
}

func (s *FileResourceService) listUploadFiles(params *form.FileResourceList, condition string, args []any) (int64, []*model.UploadFiles, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return 0, nil, err
	}
	query := db.Model(&model.UploadFiles{})
	isDeleted := uint8(0)
	if params.IsDeleted != nil {
		isDeleted = *params.IsDeleted
	}
	if isDeleted == global.Yes {
		query = query.Unscoped().Where("upload_files.deleted_at <> 0")
	}
	if condition != "" {
		query = query.Where(condition, args...)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil || total == 0 {
		return total, nil, err
	}
	page, perPage := params.Page, params.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = global.PerPage
	}
	var rows []*model.UploadFiles
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&rows).Error; err != nil {
		return total, nil, err
	}
	s.fillReferenceCounts(rows)
	s.fillObjectReuseCounts(rows)
	return total, rows, nil
}

func (s *FileResourceService) fillReferenceCounts(rows []*model.UploadFiles) {
	if len(rows) == 0 {
		return
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return
	}
	type countRow struct {
		FileID uint
		Count  int64
	}
	var counts []countRow
	if err := db.Model(&model.UploadFileReference{}).Select("file_id, COUNT(*) AS count").Where("file_id IN ?", ids).Group("file_id").Scan(&counts).Error; err != nil {
		return
	}
	countMap := make(map[uint]int64, len(counts))
	for _, item := range counts {
		countMap[item.FileID] = item.Count
	}
	for _, row := range rows {
		row.ReferenceCount = countMap[row.ID]
	}
}

func (s *FileResourceService) fillObjectReuseCounts(rows []*model.UploadFiles) {
	if len(rows) == 0 {
		return
	}
	objectIDs := make([]uint, 0, len(rows))
	seen := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if row.FileObjectID == 0 {
			continue
		}
		if _, ok := seen[row.FileObjectID]; ok {
			continue
		}
		seen[row.FileObjectID] = struct{}{}
		objectIDs = append(objectIDs, row.FileObjectID)
	}
	if len(objectIDs) == 0 {
		return
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return
	}
	type countRow struct {
		FileObjectID uint
		Count        int64
	}
	var counts []countRow
	if err := db.Unscoped().Model(&model.UploadFiles{}).Select("file_object_id, COUNT(*) AS count").Where("file_object_id IN ?", objectIDs).Group("file_object_id").Scan(&counts).Error; err != nil {
		return
	}
	countMap := make(map[uint]int64, len(counts))
	for _, item := range counts {
		countMap[item.FileObjectID] = item.Count
	}
	var objects []model.UploadFileObject
	if err := db.Where("id IN ?", objectIDs).Find(&objects).Error; err != nil {
		return
	}
	statusMap := make(map[uint]string, len(objects))
	for _, object := range objects {
		statusMap[object.ID] = object.Status
	}
	for _, row := range rows {
		row.ObjectReuseCount = countMap[row.FileObjectID]
		row.ObjectStatus = statusMap[row.FileObjectID]
	}
}

func (s *FileResourceService) markDeleteFailed(id uint) error {
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	return db.Unscoped().Model(&model.UploadFiles{}).Where("id = ?", id).Updates(map[string]any{
		"storage_status": model.StorageStatusDeleteFailed,
		"updated_at":     time.Now(),
	}).Error
}

func (s *FileResourceService) dbOrDefault() (*gorm.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	return model.GetDB()
}
