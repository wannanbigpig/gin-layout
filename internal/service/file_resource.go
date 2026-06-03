package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
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

type FileBatchDeleteResult struct {
	Total    int                       `json:"total"`
	Deleted  int                       `json:"deleted"`
	Failed   int                       `json:"failed"`
	Failures []*FileBatchDeleteFailure `json:"failures,omitempty"`
}

type FileBatchDeleteFailure struct {
	ID         uint                                `json:"id"`
	Code       int                                 `json:"code,omitempty"`
	Message    string                              `json:"message"`
	References []*resources.FileReferenceResources `json:"references,omitempty"`
}

type FileBatchDestroyResult struct {
	Total     int                        `json:"total"`
	Destroyed int                        `json:"destroyed"`
	Failed    int                        `json:"failed"`
	Failures  []*FileBatchDestroyFailure `json:"failures,omitempty"`
}

type FileBatchDestroyFailure struct {
	ID      uint   `json:"id"`
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
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
		AddLike("mime_type", params.MimeType)
	if types := normalizeFileTypeFilter(params.FileType); len(types) > 1 {
		query.AddCondition("file_type IN ?", types)
	} else if len(types) == 1 {
		query.AddEq("file_type", types[0])
	}
	query.AddEq("is_public", params.IsPublic).
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
	if t, ok := utils.ParseTimeBoundary(params.StartTime); ok {
		query.AddCondition("created_at >= ?", t)
	}
	if t, ok := utils.ParseTimeBoundary(params.EndTime); ok {
		query.AddCondition("created_at <= ?", t)
	}
	if params.IncludeHidden == nil || *params.IncludeHidden != global.Yes {
		query.AddCondition("NOT EXISTS (SELECT 1 FROM upload_file_folders WHERE upload_file_folders.id = upload_files.folder_id AND upload_file_folders.is_hidden = 1)")
	}
	condition, args := query.Build()

	transformer := resources.NewFileResourceTransformer()
	total, collection, err := s.listUploadFiles(params, condition, args)
	if err != nil {
		log.Logger.Error("查询文件资源列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	result := transformer.ToCollection(params.Page, params.PerPage, total, collection)
	s.fillUploaderNames(result)
	return result
}

func normalizeFileTypeFilter(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
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
		s.fillSingleUploaderName(data)
	}
	return result, nil
}

// Delete 软删除文件记录。force=true 时跳过引用检查。
func (s *FileResourceService) Delete(id uint, deletedBy uint, reason string, force ...bool) error {
	uploadFile, err := s.findByID(id)
	if err != nil {
		return err
	}
	isForced := len(force) > 0 && force[0]
	if !isForced {
		refs, err := NewFileReferenceService(s.db).ReferencesByFileID(id)
		if err != nil {
			return err
		}
		if len(refs) > 0 {
			return NewFileReferencedDeleteError(buildFileReferenceResources(refs))
		}
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

func (s *FileResourceService) BatchDelete(ids []uint, deletedBy uint, reason string) (*FileBatchDeleteResult, error) {
	result := &FileBatchDeleteResult{
		Total: len(ids),
	}
	if len(ids) == 0 {
		return result, nil
	}

	for _, id := range ids {
		if err := s.Delete(id, deletedBy, reason); err != nil {
			result.Failures = append(result.Failures, buildFileBatchDeleteFailure(id, err))
			continue
		}
		result.Deleted++
	}

	result.Failed = len(result.Failures)
	return result, nil
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

	// DB 操作包入事务：删除引用关系 + 删除文件记录，引用删除失败触发回滚。
	if err := access.RunInTransaction(db, func(tx *gorm.DB) error {
		if err := tx.Where("file_id = ?", id).Delete(&model.UploadFileReference{}).Error; err != nil {
			return err
		}
		result := tx.Unscoped().Delete(&model.UploadFiles{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return e.NewBusinessError(e.NotFound)
		}
		if uploadFile.FileObjectID > 0 {
			return s.deleteObjectIfUnreferenced(tx, uploadFile.FileObjectID)
		}
		return nil
	}); err != nil {
		return err
	}

	// 物理文件删除在事务外执行（DB 记录已删，blob 删除失败会产生磁盘孤儿）。
	// 失败不再静默吞掉：记录 ERROR 并返回错误，让调用方/监控可见，避免孤儿文件无声泄漏。
	if uploadFile.FileObjectID == 0 {
		if err := s.deletePhysicalFile(uploadFile); err != nil {
			log.Logger.Error("物理文件删除失败", zap.Error(err), zap.Uint("id", uploadFile.ID))
			return err
		}
	}
	return nil
}

func (s *FileResourceService) BatchRestore(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	result := db.Unscoped().Model(&model.UploadFiles{}).Where("id IN ? AND deleted_at <> 0", ids).Updates(map[string]any{
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

func (s *FileResourceService) BatchDestroy(ids []uint) (*FileBatchDestroyResult, error) {
	result := &FileBatchDestroyResult{
		Total: len(ids),
	}
	if len(ids) == 0 {
		return result, nil
	}

	for _, id := range ids {
		if err := s.Destroy(id); err != nil {
			result.Failures = append(result.Failures, buildFileBatchDestroyFailure(id, err))
			continue
		}
		result.Destroyed++
	}

	result.Failed = len(result.Failures)
	return result, nil
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

func buildFileBatchDeleteFailure(id uint, err error) *FileBatchDeleteFailure {
	failure := &FileBatchDeleteFailure{
		ID:      id,
		Message: firstNonEmpty(errMessage(err), e.NewBusinessError(e.FAILURE).GetMessage()),
	}

	var referencedErr *FileReferencedDeleteError
	if errors.As(err, &referencedErr) {
		if businessErr := referencedErr.BusinessError(); businessErr != nil {
			failure.Code = businessErr.GetCode()
			failure.Message = businessErr.GetMessage()
		}
		failure.References = referencedErr.References
		return failure
	}

	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) {
		failure.Code = businessErr.GetCode()
		failure.Message = businessErr.GetMessage()
	}

	return failure
}

func buildFileBatchDestroyFailure(id uint, err error) *FileBatchDestroyFailure {
	failure := &FileBatchDestroyFailure{
		ID:      id,
		Message: firstNonEmpty(errMessage(err), e.NewBusinessError(e.FAILURE).GetMessage()),
	}

	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) {
		failure.Code = businessErr.GetCode()
		failure.Message = businessErr.GetMessage()
	}

	return failure
}


func errMessage(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
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

// fillUploaderNames 批量填充列表中每条文件资源的上传人昵称/账号。
func (s *FileResourceService) fillUploaderNames(result *resources.Collection) {
	if result == nil || len(result.Data) == 0 {
		return
	}
	uidSet := make(map[uint]struct{})
	for _, item := range result.Data {
		if r, ok := item.(*resources.FileResourceResources); ok && r.UID > 0 {
			uidSet[r.UID] = struct{}{}
		}
	}
	if len(uidSet) == 0 {
		return
	}
	uids := make([]uint, 0, len(uidSet))
	for uid := range uidSet {
		uids = append(uids, uid)
	}
	nameMap := s.queryUploaderNames(uids)
	for _, item := range result.Data {
		if r, ok := item.(*resources.FileResourceResources); ok {
			if info, exists := nameMap[r.UID]; exists {
				r.UploaderName = info.Nickname
				r.UploaderUsername = info.Username
			}
		}
	}
}

// fillSingleUploaderName 填充单条文件资源详情的上传人信息。
func (s *FileResourceService) fillSingleUploaderName(r *resources.FileResourceResources) {
	if r == nil || r.UID == 0 {
		return
	}
	nameMap := s.queryUploaderNames([]uint{r.UID})
	if info, exists := nameMap[r.UID]; exists {
		r.UploaderName = info.Nickname
		r.UploaderUsername = info.Username
	}
}

type uploaderInfo struct {
	Nickname string
	Username string
}

func (s *FileResourceService) queryUploaderNames(uids []uint) map[uint]uploaderInfo {
	if len(uids) == 0 {
		return nil
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return nil
	}
	type adminRow struct {
		ID       uint
		Nickname string
		Username string
	}
	var rows []adminRow
	if err := db.Model(&model.AdminUser{}).Where("id IN ?", uids).Find(&rows).Error; err != nil {
		return nil
	}
	result := make(map[uint]uploaderInfo, len(rows))
	for _, row := range rows {
		result[row.ID] = uploaderInfo{Nickname: row.Nickname, Username: row.Username}
	}
	return result
}

func (s *FileResourceService) dbOrDefault() (*gorm.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	return model.GetDB()
}
