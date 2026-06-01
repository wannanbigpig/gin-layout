package service

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

const (
	defaultFileListExportLimit = 1000
	maxFileListExportLimit     = 50000
	fileListExportBatchSize    = 1000
)

type fileExportProgressReporter func(processed, total int) error

// GenerateFileListExportCSV 生成文件资源 CSV 数据（供异步导出任务使用）。
// 复用 FileResourceService.List 的过滤维度，但不分页，按 Limit 限制单次导出条数。
func (s *FileResourceService) GenerateFileListExportCSV(params *form.FileResourceList) ([]byte, string, error) {
	return s.generateFileListExportCSV(params, nil)
}

// GenerateFileListExportCSVWithProgress 生成文件资源导出 CSV，并在每批处理完成后回调真实进度。
func (s *FileResourceService) GenerateFileListExportCSVWithProgress(params *form.FileResourceList, reporter func(processed, total int) error) ([]byte, string, error) {
	return s.generateFileListExportCSV(params, reporter)
}

func (s *FileResourceService) generateFileListExportCSV(params *form.FileResourceList, reporter fileExportProgressReporter) ([]byte, string, error) {
	if params == nil {
		params = &form.FileResourceList{}
	}

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
	condition, args := query.Build()

	limit := normalizeFileListExportLimit(params.Limit)

	db, err := s.dbOrDefault()
	if err != nil {
		return nil, "", err
	}
	q := db.Model(&model.UploadFiles{})
	isDeleted := uint8(0)
	if params.IsDeleted != nil {
		isDeleted = *params.IsDeleted
	}
	if isDeleted == global.Yes {
		q = q.Unscoped().Where("upload_files.deleted_at <> 0")
	}
	if condition != "" {
		q = q.Where(condition, args...)
	}

	var totalRows int64
	if err := q.Count(&totalRows).Error; err != nil {
		return nil, "", err
	}
	total := int(totalRows)
	if total > limit {
		total = limit
	}

	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write([]string{
		"id",
		"uuid",
		"origin_name",
		"file_type",
		"mime_type",
		"size",
		"is_public",
		"storage_driver",
		"storage_status",
		"folder_id",
		"created_at",
	}); err != nil {
		return nil, "", err
	}

	if reporter != nil {
		if err := reporter(0, total); err != nil {
			return nil, "", err
		}
	}

	processed := 0
	for processed < total {
		batchLimit := fileListExportBatchSize
		if remaining := total - processed; remaining < batchLimit {
			batchLimit = remaining
		}
		rows := make([]*model.UploadFiles, 0, batchLimit)
		batchQuery := q.Session(&gorm.Session{}).Order("created_at DESC, id DESC").Limit(batchLimit).Offset(processed)
		if err := batchQuery.Find(&rows).Error; err != nil {
			return nil, "", err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := writer.Write([]string{
				strconv.FormatUint(uint64(row.ID), 10),
				row.UUID,
				row.OriginName,
				row.FileType,
				row.MimeType,
				strconv.FormatUint(uint64(row.Size), 10),
				strconv.Itoa(int(row.IsPublic)),
				row.StorageDriver,
				row.StorageStatus,
				strconv.FormatUint(uint64(row.FolderID), 10),
				row.CreatedAt.String(),
			}); err != nil {
				return nil, "", err
			}
		}
		processed += len(rows)
		if reporter != nil {
			if err := reporter(processed, total); err != nil {
				return nil, "", err
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", err
	}

	fileName := "file_resources_" + time.Now().Format("20060102150405") + ".csv"
	return buffer.Bytes(), fileName, nil
}

// normalizeFileListExportLimit 将 limit 规整到合理区间，避免任务跑飞。
func normalizeFileListExportLimit(limit int) int {
	if limit <= 0 {
		return defaultFileListExportLimit
	}
	if limit > maxFileListExportLimit {
		return maxFileListExportLimit
	}
	return limit
}
