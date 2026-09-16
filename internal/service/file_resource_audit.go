package service

import (
	"sort"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// FileAuditDiff 是同一事务内读取的业务变更前后快照。
type FileAuditDiff[T any] struct {
	Before T
	After  T
}

func runFileAuditTransaction[Result any, Snapshot any](
	service *FileResourceService,
	lock func(tx *gorm.DB) error,
	snapshot func(service *FileResourceService) (Snapshot, error),
	mutate func(service *FileResourceService) (Result, error),
) (Result, FileAuditDiff[Snapshot], error) {
	var result Result
	var diff FileAuditDiff[Snapshot]
	db, err := service.dbOrDefault()
	if err != nil {
		return result, diff, err
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		if err := lock(tx); err != nil {
			return err
		}
		txService := *service
		txService.db = tx
		before, err := snapshot(&txService)
		if err != nil {
			return err
		}
		result, err = mutate(&txService)
		if err != nil {
			return err
		}
		after, err := snapshot(&txService)
		if err != nil {
			return err
		}
		diff = FileAuditDiff[Snapshot]{Before: before, After: after}
		return nil
	})
	return result, diff, err
}

// UpdateFileWithAudit 在一笔事务内更新文件并返回一致的审计快照。
func (s *FileResourceService) UpdateFileWithAudit(params *form.FileUpdate) (any, FileAuditDiff[[]FileAuditSnapshot], error) {
	ids := []uint{params.ID}
	return runFileAuditTransaction(
		s,
		func(tx *gorm.DB) error { return lockFileAuditRows(tx, ids) },
		func(txService *FileResourceService) ([]FileAuditSnapshot, error) {
			return txService.FileAuditSnapshots(ids)
		},
		func(txService *FileResourceService) (any, error) {
			return txService.UpdateFile(params)
		},
	)
}

// UpdateFolderWithAudit 在一笔事务内更新目录并返回一致的审计快照。
func (s *FileResourceService) UpdateFolderWithAudit(params *form.FileFolderUpdate, uid uint) (*resources.FileFolderResources, FileAuditDiff[*FileFolderAuditSnapshot], error) {
	return runFileAuditTransaction(
		s,
		func(tx *gorm.DB) error { return lockFileFolderAuditRows(tx, params.ID) },
		func(txService *FileResourceService) (*FileFolderAuditSnapshot, error) {
			return txService.FileFolderAuditSnapshot(params.ID)
		},
		func(txService *FileResourceService) (*resources.FileFolderResources, error) {
			return txService.UpdateFolder(params, uid)
		},
	)
}

// MoveFolderWithAudit 在一笔事务内移动目录并返回一致的审计快照。
func (s *FileResourceService) MoveFolderWithAudit(params *form.FileFolderMove, uid uint) (*resources.FileFolderResources, FileAuditDiff[*FileFolderAuditSnapshot], error) {
	targetParentID := params.ParentID
	if params.TargetParentID > 0 {
		targetParentID = params.TargetParentID
	}
	return runFileAuditTransaction(
		s,
		func(tx *gorm.DB) error { return lockFileFolderAuditRows(tx, params.ID, targetParentID) },
		func(txService *FileResourceService) (*FileFolderAuditSnapshot, error) {
			return txService.FileFolderAuditSnapshot(params.ID)
		},
		func(txService *FileResourceService) (*resources.FileFolderResources, error) {
			return txService.MoveFolder(params, uid)
		},
	)
}

// MoveFilesWithAudit 在一笔事务内移动文件并返回一致的审计快照。
func (s *FileResourceService) MoveFilesWithAudit(params *form.FileMove) (*resources.FileMoveResult, FileAuditDiff[[]FileAuditSnapshot], error) {
	return runFileAuditTransaction(
		s,
		func(tx *gorm.DB) error { return lockFileAuditRows(tx, params.IDs) },
		func(txService *FileResourceService) ([]FileAuditSnapshot, error) {
			return txService.FileAuditSnapshots(params.IDs)
		},
		func(txService *FileResourceService) (*resources.FileMoveResult, error) {
			return txService.MoveFiles(params)
		},
	)
}

func lockFileAuditRows(tx *gorm.DB, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var rows []model.UploadFiles
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id IN ?", ids).
		Order("id ASC").
		Find(&rows).Error
}

func lockFileFolderAuditRows(tx *gorm.DB, ids ...uint) error {
	ids = sortedPositiveUniqueIDs(ids)
	if len(ids) == 0 {
		return nil
	}
	var folders []model.UploadFileFolder
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id IN ?", ids).
		Order("id ASC").
		Find(&folders).Error
}

func sortedPositiveUniqueIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
