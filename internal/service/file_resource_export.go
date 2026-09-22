package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"strconv"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/csvutil"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"gorm.io/gorm"
)

const (
	defaultFileListExportLimit = 1000
	maxFileListExportLimit     = 50000
	fileListExportBatchSize    = 1000
)

type fileExportProgressReporter func(processed, total int) error

// HasFileListExportRows 使用与流式导出相同的过滤条件，在创建异步任务前快速判断是否为空。
func (s *FileResourceService) HasFileListExportRows(ctx context.Context, params *form.FileResourceList) (bool, error) {
	query, err := s.fileListExportQuery(ctx, params)
	if err != nil {
		return false, err
	}
	var rows []model.UploadFiles
	result := query.Select("upload_files.id").Limit(1).Find(&rows)
	return result.RowsAffected > 0, result.Error
}

func (s *FileResourceService) fileListExportQuery(ctx context.Context, params *form.FileResourceList) (*gorm.DB, error) {
	if params == nil {
		params = &form.FileResourceList{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	condition, args, err := s.buildFileResourceFilter(params)
	if err != nil {
		return nil, err
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	query := db.WithContext(ctx).Model(&model.UploadFiles{})
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
	return query, nil
}

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
	buffer := &bytes.Buffer{}
	if err := s.StreamFileListExportCSV(context.Background(), params, buffer, reporter); err != nil {
		return nil, "", err
	}
	fileName := "file_resources_" + time.Now().Format("20060102150405") + ".csv"
	return buffer.Bytes(), fileName, nil
}

// StreamFileListExportCSV 使用稳定游标分批写出 CSV，避免 OFFSET 扫描和整份结果驻留内存。
func (s *FileResourceService) StreamFileListExportCSV(ctx context.Context, params *form.FileResourceList, output io.Writer, reporter fileExportProgressReporter) error {
	if params == nil {
		params = &form.FileResourceList{}
	}
	if output == nil {
		return io.ErrClosedPipe
	}
	if ctx == nil {
		ctx = context.Background()
	}
	q, err := s.fileListExportQuery(ctx, params)
	if err != nil {
		return err
	}

	limit := normalizeFileListExportLimit(params.Limit)

	var totalRows int64
	if err := q.Count(&totalRows).Error; err != nil {
		return err
	}
	total := int(totalRows)
	if total > limit {
		total = limit
	}

	writer := csv.NewWriter(output)
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
		return err
	}

	if reporter != nil {
		if err := reporter(0, total); err != nil {
			return err
		}
	}

	processed := 0
	var cursorCreatedAt time.Time
	var cursorID uint
	for processed < total {
		if err := ctx.Err(); err != nil {
			return err
		}
		batchLimit := fileListExportBatchSize
		if remaining := total - processed; remaining < batchLimit {
			batchLimit = remaining
		}
		rows := make([]*model.UploadFiles, 0, batchLimit)
		batchQuery := q.Session(&gorm.Session{}).Order("created_at DESC, id DESC").Limit(batchLimit)
		if !cursorCreatedAt.IsZero() {
			batchQuery = batchQuery.Where("created_at < ? OR (created_at = ? AND id < ?)", cursorCreatedAt, cursorCreatedAt, cursorID)
		}
		if err := batchQuery.Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := writer.Write([]string{
				strconv.FormatUint(uint64(row.ID), 10),
				csvutil.SanitizeCell(row.UUID),
				csvutil.SanitizeCell(row.OriginName),
				csvutil.SanitizeCell(row.FileType),
				csvutil.SanitizeCell(row.MimeType),
				strconv.FormatUint(uint64(row.Size), 10),
				strconv.Itoa(int(row.IsPublic)),
				csvutil.SanitizeCell(row.StorageDriver),
				csvutil.SanitizeCell(row.StorageStatus),
				strconv.FormatUint(uint64(row.FolderID), 10),
				row.CreatedAt.String(),
			}); err != nil {
				return err
			}
		}
		processed += len(rows)
		last := rows[len(rows)-1]
		cursorCreatedAt = last.CreatedAt.Time
		cursorID = last.ID
		if reporter != nil {
			if err := reporter(processed, total); err != nil {
				return err
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	return nil
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
