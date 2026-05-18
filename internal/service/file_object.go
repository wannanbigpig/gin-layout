package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

type uploadFileObjectInput struct {
	StorageDriver string
	StorageBase   string
	Bucket        string
	StoragePath   string
	ObjectKey     string
	Size          uint
	Hash          string
	MimeType      string
	ETag          string
	Status        string
}

func findReusableFileObject(tx *gorm.DB, storageDriver, bucket, hash string) (*model.UploadFileObject, error) {
	if hash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	bucket = normalizeFileObjectBucket(storageDriver, bucket)
	var object model.UploadFileObject
	query := tx.Where("storage_driver = ? AND bucket = ? AND hash = ? AND status = ?", storageDriver, bucket, hash, model.StorageStatusStored)
	if err := query.Order("id ASC").First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func findFileObjectByID(tx *gorm.DB, id uint) (*model.UploadFileObject, error) {
	if id == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var object model.UploadFileObject
	if err := tx.First(&object, id).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func createFileObject(tx *gorm.DB, input uploadFileObjectInput) (*model.UploadFileObject, error) {
	status := input.Status
	if status == "" {
		status = model.StorageStatusStored
	}
	bucket := normalizeFileObjectBucket(input.StorageDriver, input.Bucket)
	object := &model.UploadFileObject{
		StorageDriver: input.StorageDriver,
		StorageBase:   input.StorageBase,
		Bucket:        bucket,
		StoragePath:   firstNonEmpty(input.StoragePath, input.ObjectKey),
		ObjectKey:     firstNonEmpty(input.ObjectKey, input.StoragePath),
		Size:          input.Size,
		Hash:          input.Hash,
		MimeType:      input.MimeType,
		ETag:          input.ETag,
		Status:        status,
	}
	if err := tx.Create(object).Error; err != nil {
		if existing, findErr := findReusableFileObject(tx, input.StorageDriver, bucket, input.Hash); findErr == nil {
			return existing, nil
		}
		return nil, err
	}
	return object, nil
}

func ensureFileObject(tx *gorm.DB, input uploadFileObjectInput) (*model.UploadFileObject, bool, error) {
	if object, err := findReusableFileObject(tx, input.StorageDriver, input.Bucket, input.Hash); err == nil {
		return object, true, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	object, err := createFileObject(tx, input)
	return object, false, err
}

func normalizeFileObjectBucket(storageDriver, bucket string) string {
	if storageDriver == model.StorageDriverLocal {
		return ""
	}
	return bucket
}

func applyObjectToUploadFile(uploadFile *model.UploadFiles, object *model.UploadFileObject) {
	if uploadFile == nil || object == nil {
		return
	}
	uploadFile.FileObjectID = object.ID
	uploadFile.StorageDriver = object.StorageDriver
	uploadFile.StorageBase = object.StorageBase
	uploadFile.Bucket = object.Bucket
	uploadFile.StoragePath = object.StoragePath
	uploadFile.ObjectKey = object.ObjectKey
	uploadFile.ETag = object.ETag
	uploadFile.StorageStatus = object.Status
	if uploadFile.Path == "" {
		uploadFile.Path = object.ObjectKey
	}
	if uploadFile.Hash == "" {
		uploadFile.Hash = object.Hash
	}
	if uploadFile.Size == 0 {
		uploadFile.Size = object.Size
	}
	if uploadFile.MimeType == "" {
		uploadFile.MimeType = object.MimeType
	}
}

func (s *FileResourceService) deletePhysicalObject(object *model.UploadFileObject) error {
	if object == nil {
		return nil
	}
	driverName := firstNonEmpty(object.StorageDriver, model.StorageDriverLocal)
	var driver filestorage.Driver
	var err error
	if driverName == model.StorageDriverLocal && object.StorageBase != "" {
		driver = filestorage.NewLocalDriver(filestorage.LocalConfig{
			PublicBasePath:  object.StorageBase,
			PrivateBasePath: object.StorageBase,
		}, object.StorageBase, object.StorageBase)
	} else {
		driver, _, err = s.storageDriverByName(context.Background(), driverName)
	}
	if err != nil {
		return err
	}
	objectKey := firstNonEmpty(object.ObjectKey, object.StoragePath)
	if objectKey == "" {
		return nil
	}
	return driver.Delete(context.Background(), object.Bucket, objectKey)
}

func (s *FileResourceService) deleteObjectIfUnreferenced(db *gorm.DB, objectID uint) error {
	if objectID == 0 {
		return nil
	}
	var count int64
	if err := db.Unscoped().Model(&model.UploadFiles{}).Where("file_object_id = ?", objectID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	object, err := findFileObjectByID(db, objectID)
	if err != nil {
		return err
	}
	if err := s.deletePhysicalObject(object); err != nil {
		_ = db.Model(&model.UploadFileObject{}).Where("id = ?", objectID).Updates(map[string]any{
			"status":     model.StorageStatusDeleteFailed,
			"updated_at": time.Now(),
		}).Error
		return fmt.Errorf("delete physical object: %w", err)
	}
	return db.Delete(&model.UploadFileObject{}, objectID).Error
}
