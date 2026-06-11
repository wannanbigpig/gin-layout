package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	pkgErrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	dateutils "github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

type uploadFileObjectInput struct {
	StorageDriver string
	StorageBase   string
	Bucket        string
	StoragePath   string
	ObjectKey     string
	OwnerUID      uint
	IsPublic      uint8
	Size          uint
	Hash          string
	MimeType      string
	ETag          string
	Status        string
}

func findReusableFileObject(tx *gorm.DB, storageDriver, bucket, hash string, ownerUID uint, isPublic uint8) (*model.UploadFileObject, error) {
	if hash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	bucket = normalizeFileObjectBucket(storageDriver, bucket)
	ownerUID = normalizeFileObjectOwnerUID(ownerUID, isPublic)
	var object model.UploadFileObject
	query := tx.Where("storage_driver = ? AND bucket = ? AND hash = ? AND owner_uid = ? AND is_public = ? AND status = ?",
		storageDriver, bucket, hash, ownerUID, isPublic, model.StorageStatusStored)
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
	ownerUID := normalizeFileObjectOwnerUID(input.OwnerUID, input.IsPublic)
	object := &model.UploadFileObject{
		StorageDriver: input.StorageDriver,
		StorageBase:   input.StorageBase,
		Bucket:        bucket,
		StoragePath:   firstNonEmpty(input.StoragePath, input.ObjectKey),
		ObjectKey:     firstNonEmpty(input.ObjectKey, input.StoragePath),
		OwnerUID:      ownerUID,
		IsPublic:      input.IsPublic,
		Size:          input.Size,
		Hash:          input.Hash,
		MimeType:      input.MimeType,
		ETag:          input.ETag,
		Status:        status,
	}
	if err := tx.Create(object).Error; err != nil {
		if existing, findErr := findReusableFileObject(tx, input.StorageDriver, bucket, input.Hash, input.OwnerUID, input.IsPublic); findErr == nil {
			return existing, nil
		}
		return nil, err
	}
	return object, nil
}

func ensureFileObject(tx *gorm.DB, input uploadFileObjectInput) (*model.UploadFileObject, bool, error) {
	if object, err := findReusableFileObject(tx, input.StorageDriver, input.Bucket, input.Hash, input.OwnerUID, input.IsPublic); err == nil {
		return object, true, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	object, err := createFileObject(tx, input)
	return object, false, err
}

type uploadSessionInput struct {
	UploadType    string
	StorageDriver string
	StorageBase   string
	Bucket        string
	StoragePath   string
	ObjectKey     string
	UploadID      string
	FileObjectID  uint
	OwnerUID      uint
	FolderID      uint
	OriginName    string
	Size          uint
	Hash          string
	MimeType      string
	ETag          string
	IsPublic      uint8
	UploadScene   string
	ExpiresAt     time.Time
}

func createUploadSession(tx *gorm.DB, input uploadSessionInput) (string, *model.UploadSession, error) {
	if tx == nil {
		return "", nil, gorm.ErrInvalidDB
	}
	completeToken, tokenHash, err := newUploadCompleteToken()
	if err != nil {
		return "", nil, err
	}
	status := model.UploadSessionStatusPending
	expiresAt := input.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(defaultUploadSessionTTL)
	}
	session := &model.UploadSession{
		CompleteTokenHash: tokenHash,
		UploadType:        input.UploadType,
		Status:            status,
		StorageDriver:     input.StorageDriver,
		StorageBase:       input.StorageBase,
		Bucket:            normalizeFileObjectBucket(input.StorageDriver, input.Bucket),
		StoragePath:       firstNonEmpty(input.StoragePath, input.ObjectKey),
		ObjectKey:         firstNonEmpty(input.ObjectKey, input.StoragePath),
		UploadID:          input.UploadID,
		FileObjectID:      input.FileObjectID,
		OwnerUID:          input.OwnerUID,
		FolderID:          input.FolderID,
		OriginName:        input.OriginName,
		Size:              input.Size,
		Hash:              input.Hash,
		MimeType:          input.MimeType,
		ETag:              input.ETag,
		IsPublic:          input.IsPublic,
		UploadScene:       input.UploadScene,
		ExpiresAt:         dateutils.FormatDate{Time: expiresAt},
	}
	if err := tx.Create(session).Error; err != nil {
		return "", nil, err
	}
	return completeToken, session, nil
}

func consumeUploadSession(tx *gorm.DB, completeToken string, ownerUID uint, allowedTypes ...string) (*model.UploadSession, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}
	tokenHash := uploadCompleteTokenHash(completeToken)
	if tokenHash == "" {
		return nil, fmt.Errorf("complete_token is required")
	}
	query := tx.Clauses(clauseLockForUpdate()).Where("complete_token_hash = ? AND owner_uid = ?", tokenHash, ownerUID)
	if len(allowedTypes) > 0 {
		query = query.Where("upload_type IN ?", allowedTypes)
	}
	var session model.UploadSession
	if err := query.First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("upload session not found")
		}
		return nil, err
	}
	if session.Status != model.UploadSessionStatusPending {
		return nil, fmt.Errorf("upload session is not pending")
	}
	if !session.ExpiresAt.IsZero() && time.Now().After(session.ExpiresAt.Time) {
		if err := tx.Model(&model.UploadSession{}).Where("id = ?", session.ID).Updates(map[string]any{
			"status":     model.UploadSessionStatusExpired,
			"updated_at": time.Now(),
		}).Error; err != nil {
			log.Logger.Warn("更新上传会话过期状态失败", zap.Uint("session_id", session.ID), zap.Error(err))
		}
		return nil, fmt.Errorf("upload session expired")
	}
	return &session, nil
}

func findPendingUploadSession(tx *gorm.DB, token string, uploadType string, uid uint) (*model.UploadSession, error) {
	session, err := consumeUploadSession(tx, token, uid, uploadType)
	if err != nil {
		return nil, pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, err.Error())
	}
	return session, nil
}

func findCompletingUploadSession(tx *gorm.DB, sessionID uint, uid uint, uploadType string) (*model.UploadSession, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}
	var session model.UploadSession
	if err := tx.Clauses(clauseLockForUpdate()).
		Where("id = ? AND owner_uid = ? AND upload_type = ?", sessionID, uid, uploadType).
		First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "upload session not found")
		}
		return nil, err
	}
	if session.Status != model.UploadSessionStatusCompleting {
		return nil, pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "complete_token is already used")
	}
	return &session, nil
}

func markUploadSessionCompleted(tx *gorm.DB, session *model.UploadSession, fileID uint, objectID uint) error {
	if tx == nil || session == nil {
		return nil
	}
	now := time.Now()
	res := tx.Model(&model.UploadSession{}).
		Where("id = ? AND status IN ?", session.ID, []string{model.UploadSessionStatusPending, model.UploadSessionStatusCompleting}).
		Updates(map[string]any{
			"status":         model.UploadSessionStatusCompleted,
			"file_id":        fileID,
			"file_object_id": objectID,
			"completed_at":   now,
			"updated_at":     now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "complete_token is already used")
	}
	return nil
}

func claimUploadSessionCompleting(tx *gorm.DB, session *model.UploadSession) error {
	if tx == nil || session == nil {
		return nil
	}
	res := tx.Model(&model.UploadSession{}).
		Where("id = ? AND status = ?", session.ID, model.UploadSessionStatusPending).
		Updates(map[string]any{
			"status":     model.UploadSessionStatusCompleting,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "complete_token is already used")
	}
	session.Status = model.UploadSessionStatusCompleting
	return nil
}

func restoreUploadSessionPending(ctx context.Context, db *gorm.DB, sessionID uint) {
	if db == nil || sessionID == 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := db.WithContext(ctx).Model(&model.UploadSession{}).
		Where("id = ? AND status = ?", sessionID, model.UploadSessionStatusCompleting).
		Updates(map[string]any{
			"status":     model.UploadSessionStatusPending,
			"updated_at": time.Now(),
		}).Error; err != nil {
		log.Logger.Warn("恢复上传会话为 pending 状态失败", zap.Uint("session_id", sessionID), zap.Error(err))
	}
}

func validateDirectCompleteSession(session *model.UploadSession, params *form.FileUploadComplete, uid uint) error {
	if session == nil || params == nil {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "complete_token is invalid")
	}
	if session.OwnerUID != uid {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "complete_token owner mismatch")
	}
	if err := validateSessionCommonFields(session, params.FolderID, params.OriginName, params.Size, params.Hash, params.MimeType, params.IsPublic); err != nil {
		return err
	}
	if driver := firstNonEmpty(params.StorageDriver, params.Driver); strings.TrimSpace(driver) != "" && driver != session.StorageDriver {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "storage driver does not match upload session")
	}
	if strings.TrimSpace(params.Bucket) != "" && normalizeFileObjectBucket(session.StorageDriver, params.Bucket) != session.Bucket {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "bucket does not match upload session")
	}
	if strings.TrimSpace(params.ObjectKey) != "" && strings.TrimSpace(params.ObjectKey) != session.ObjectKey {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "object_key does not match upload session")
	}
	if params.FileObjectID > 0 && params.FileObjectID != session.FileObjectID {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file_object_id does not match upload session")
	}
	return nil
}

func validateMultipartCompleteSession(session *model.UploadSession, params *form.FileMultipartComplete, uid uint) error {
	if session == nil || params == nil {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "complete_token is invalid")
	}
	if session.OwnerUID != uid {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "complete_token owner mismatch")
	}
	if err := validateSessionCommonFields(session, params.FolderID, params.OriginName, params.Size, params.Hash, params.MimeType, params.IsPublic); err != nil {
		return err
	}
	if strings.TrimSpace(params.Driver) != "" && strings.TrimSpace(params.Driver) != session.StorageDriver {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "storage driver does not match upload session")
	}
	if strings.TrimSpace(params.Bucket) != "" && normalizeFileObjectBucket(session.StorageDriver, params.Bucket) != session.Bucket {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "bucket does not match upload session")
	}
	if strings.TrimSpace(params.ObjectKey) != "" && strings.TrimSpace(params.ObjectKey) != session.ObjectKey {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "object_key does not match upload session")
	}
	if strings.TrimSpace(params.UploadID) != "" && strings.TrimSpace(params.UploadID) != session.UploadID {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "upload_id does not match upload session")
	}
	if params.FileObjectID > 0 && params.FileObjectID != session.FileObjectID {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file_object_id does not match upload session")
	}
	return nil
}

func validateSessionCommonFields(session *model.UploadSession, folderID uint, originName string, size uint, hash string, mimeType string, isPublic uint8) error {
	if folderID > 0 && folderID != session.FolderID {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "folder_id does not match upload session")
	}
	if value := strings.TrimSpace(originName); value != "" && value != session.OriginName {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "origin_name does not match upload session")
	}
	if size > 0 && size != session.Size {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "size does not match upload session")
	}
	if value := strings.TrimSpace(hash); value != "" && value != session.Hash {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "hash does not match upload session")
	}
	if value := strings.TrimSpace(mimeType); value != "" && value != session.MimeType {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "mime_type does not match upload session")
	}
	if isPublic != session.IsPublic {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "is_public does not match upload session")
	}
	return nil
}

func requireUploadSessionObject(session *model.UploadSession) error {
	if session == nil {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "complete_token is invalid")
	}
	if strings.TrimSpace(session.StorageDriver) == "" {
		return fmt.Errorf("storage driver is required")
	}
	if strings.TrimSpace(session.ObjectKey) == "" {
		return fmt.Errorf("object_key is required")
	}
	return nil
}

func validateFileObjectMatchesSession(object *model.UploadFileObject, session *model.UploadSession) error {
	if object == nil || session == nil {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file object does not match upload session")
	}
	if object.StorageDriver != session.StorageDriver {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file object storage driver mismatch")
	}
	if normalizeFileObjectBucket(object.StorageDriver, object.Bucket) != session.Bucket {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file object bucket mismatch")
	}
	if strings.TrimSpace(object.ObjectKey) != session.ObjectKey {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file object key mismatch")
	}
	if object.Hash != session.Hash || object.Size != session.Size || object.MimeType != session.MimeType {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file object metadata mismatch")
	}
	if object.IsPublic != session.IsPublic {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file object visibility mismatch")
	}
	if normalizeFileObjectOwnerUID(object.OwnerUID, object.IsPublic) != normalizeFileObjectOwnerUID(session.OwnerUID, session.IsPublic) {
		return pkgErrors.NewBusinessError(pkgErrors.InvalidParameter, "file object owner mismatch")
	}
	return nil
}

const defaultUploadSessionTTL = 15 * time.Minute

func newUploadCompleteToken() (string, string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(randomBytes)
	return token, uploadCompleteTokenHash(token), nil
}

func uploadCompleteTokenHash(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func clauseLockForUpdate() clause.Locking {
	return clause.Locking{Strength: "UPDATE"}
}

func normalizeFileObjectBucket(storageDriver, bucket string) string {
	if storageDriver == model.StorageDriverLocal {
		return ""
	}
	return bucket
}

func normalizeFileObjectOwnerUID(ownerUID uint, isPublic uint8) uint {
	if isPublic == global.Yes {
		return 0
	}
	return ownerUID
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
		if updateErr := db.Model(&model.UploadFileObject{}).Where("id = ?", objectID).Updates(map[string]any{
			"status":     model.StorageStatusDeleteFailed,
			"updated_at": time.Now(),
		}).Error; updateErr != nil {
			log.Logger.Warn("更新文件对象删除失败状态失败", zap.Uint("object_id", objectID), zap.Error(updateErr))
		}
		return fmt.Errorf("delete physical object: %w", err)
	}
	return db.Delete(&model.UploadFileObject{}, objectID).Error
}
