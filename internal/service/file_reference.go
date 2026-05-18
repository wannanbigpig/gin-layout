package service

import (
	"net/url"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type FileReferenceService struct {
	db *gorm.DB
}

func NewFileReferenceService(tx ...*gorm.DB) *FileReferenceService {
	var db *gorm.DB
	if len(tx) > 0 {
		db = tx[0]
	}
	return &FileReferenceService{db: db}
}

func (s *FileReferenceService) BindReference(fileURL, ownerType string, ownerID uint, ownerField string) error {
	uuid := ExtractFileUUID(fileURL)
	if uuid == "" || ownerType == "" || ownerID == 0 || ownerField == "" {
		return nil
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.UploadFileReference{}) {
		return nil
	}
	var file model.UploadFiles
	if err := db.Where("uuid = ? AND deleted_at = 0", uuid).First(&file).Error; err != nil {
		return nil
	}
	row := model.UploadFileReference{
		FileID:     file.ID,
		UUID:       file.UUID,
		OwnerType:  ownerType,
		OwnerID:    ownerID,
		OwnerField: ownerField,
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_type"}, {Name: "owner_id"}, {Name: "owner_field"}, {Name: "file_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"uuid", "updated_at"}),
	}).Create(&row).Error
}

func (s *FileReferenceService) ReleaseReference(fileURL, ownerType string, ownerID uint, ownerField string) error {
	uuid := ExtractFileUUID(fileURL)
	if uuid == "" {
		return nil
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.UploadFileReference{}) {
		return nil
	}
	return db.Where("uuid = ? AND owner_type = ? AND owner_id = ? AND owner_field = ?", uuid, ownerType, ownerID, ownerField).Delete(&model.UploadFileReference{}).Error
}

func (s *FileReferenceService) ReleaseReferencesByOwner(ownerType string, ownerID uint, ownerField string) error {
	db, err := s.dbOrDefault()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.UploadFileReference{}) {
		return nil
	}
	query := db.Where("owner_type = ? AND owner_id = ?", ownerType, ownerID)
	if ownerField != "" {
		query = query.Where("owner_field = ?", ownerField)
	}
	return query.Delete(&model.UploadFileReference{}).Error
}

func (s *FileReferenceService) HasActiveReferences(fileID uint) (bool, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return false, err
	}
	if !db.Migrator().HasTable(&model.UploadFileReference{}) {
		return false, nil
	}
	var count int64
	if err := db.Model(&model.UploadFileReference{}).Where("file_id = ?", fileID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *FileReferenceService) List(params *form.FileReferenceList) *resources.Collection {
	query := buildFileReferenceListQuery(params)
	condition, args := query.Build()
	modelRef := model.NewUploadFileReference()
	if s.db != nil {
		modelRef.SetDB(s.db)
	}
	db, err := s.dbOrDefault()
	if err != nil || !db.Migrator().HasTable(&model.UploadFileReference{}) {
		return resources.NewFileReferenceTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}
	transformer := resources.NewFileReferenceTransformer()
	total, rows, err := model.ListPageE(modelRef, params.Page, params.PerPage, condition, args, model.ListOptionalParams{OrderBy: "created_at DESC, id DESC"})
	if err != nil {
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, rows)
}

func buildFileReferenceListQuery(params *form.FileReferenceList) *query_builder.QueryBuilder {
	if params == nil {
		return query_builder.New()
	}
	fileID := referenceListFileID(params)
	query := query_builder.New().
		AddLike("uuid", params.UUID).
		AddEq("owner_type", params.OwnerType).
		AddEq("owner_field", params.OwnerField)
	if fileID > 0 {
		query.AddEq("file_id", fileID)
	}
	if params.OwnerID > 0 {
		query.AddEq("owner_id", params.OwnerID)
	}
	return query
}

func referenceListFileID(params *form.FileReferenceList) uint {
	if params == nil {
		return 0
	}
	if params.FileID > 0 {
		return params.FileID
	}
	return params.ID
}

func (s *FileReferenceService) ReferencesByFileID(fileID uint) ([]*model.UploadFileReference, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	if !db.Migrator().HasTable(&model.UploadFileReference{}) {
		return nil, nil
	}
	var rows []*model.UploadFileReference
	err = db.Where("file_id = ?", fileID).Order("created_at DESC, id DESC").Find(&rows).Error
	return rows, err
}

func ExtractFileUUID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Path != "" {
		raw = parsed.Path
	}
	parts := strings.Split(strings.TrimRight(raw, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	uuid := parts[len(parts)-1]
	if len(uuid) != 32 {
		return ""
	}
	return uuid
}

func (s *FileReferenceService) dbOrDefault() (*gorm.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	return model.GetDB()
}
