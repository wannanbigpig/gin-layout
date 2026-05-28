package audit

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
	"github.com/wannanbigpig/gin-layout/internal/service/sys_config"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

const (
	defaultRequestLogExportLimit = 1000
	requestLogExportBatchSize    = 500
)

type exportProgressReporter func(processed, total int) error

const (
	requestLogMaskConfigGroupCode = "audit"
	requestLogMaskConfigSort      = 95
	requestLogMaskConfigRemark    = "请求日志脱敏字段配置"
	requestLogMaskConfigManageTab = "audit_mask"
)

var requestLogMaskConfigNameI18n = map[string]string{
	"zh-CN": "请求日志脱敏配置",
	"en-US": "Request Log Mask Config",
}

var requestLogMaskConfigDiffRules = []auditdiff.FieldRule{
	{Field: "common", Label: "通用脱敏字段"},
	{Field: "request_header", Label: "请求头脱敏字段"},
	{Field: "request_body", Label: "请求体脱敏字段"},
	{Field: "response_header", Label: "响应头脱敏字段"},
	{Field: "response_body", Label: "响应体脱敏字段"},
}

// ExportCSV 导出请求日志 CSV。
func (s *RequestLogService) ExportCSV(params *form.RequestLogExport) ([]byte, string, error) {
	return generateRequestLogExportCSVBytes(params, nil)
}

// GetMaskConfig 获取当前敏感字段脱敏配置。
func (s *RequestLogService) GetMaskConfig() map[string]any {
	cfg, err := loadMaskConfigFromSysConfig()
	if err != nil {
		cfg = sensitive.GetSensitiveFieldsConfig()
	}
	return toMaskConfigMap(cfg)
}

func toMaskConfigMap(cfg sensitive.SensitiveFieldsConfig) map[string]any {
	return map[string]any{
		"common":          cfg.Common,
		"request_header":  cfg.RequestHeader,
		"request_body":    cfg.RequestBody,
		"response_header": cfg.ResponseHeader,
		"response_body":   cfg.ResponseBody,
	}
}

// UpdateMaskConfig 更新敏感字段脱敏配置（运行时生效）。
func (s *RequestLogService) UpdateMaskConfig(params *form.RequestLogMaskConfigForm) (map[string]any, error) {
	if params == nil {
		return nil, e.NewBusinessError(e.InvalidParameter)
	}

	nextConfig := sensitive.SensitiveFieldsConfig{
		Common:         normalizeSensitiveFieldList(params.Common),
		RequestHeader:  normalizeSensitiveFieldList(params.RequestHeader),
		RequestBody:    normalizeSensitiveFieldList(params.RequestBody),
		ResponseHeader: normalizeSensitiveFieldList(params.ResponseHeader),
		ResponseBody:   normalizeSensitiveFieldList(params.ResponseBody),
	}
	nextConfig = sensitive.NormalizeSensitiveFieldsConfig(nextConfig)

	persisted, err := saveMaskConfigToSysConfig(nextConfig)
	if err != nil {
		return nil, err
	}
	if persisted {
		if err := sys_config.NewSysConfigService().RefreshCache(); err != nil {
			return nil, err
		}
		return s.GetMaskConfig(), nil
	}

	// 兼容 sys_config 表尚未初始化的场景，保持运行时即时生效。
	sensitive.LoadSensitiveFieldsConfig(nextConfig)
	return toMaskConfigMap(nextConfig), nil
}

// UpdateMaskConfigWithAuditDiff 更新脱敏配置并返回精确 change_diff JSON。
func (s *RequestLogService) UpdateMaskConfigWithAuditDiff(params *form.RequestLogMaskConfigForm) (map[string]any, string, error) {
	before := s.GetMaskConfig()
	after, err := s.UpdateMaskConfig(params)
	if err != nil {
		return nil, "", err
	}
	changeDiff := buildMaskConfigAuditDiff(before, after)
	return after, changeDiff, nil
}

func buildMaskConfigAuditDiff(before, after map[string]any) string {
	items := auditdiff.BuildFieldDiff(before, after, requestLogMaskConfigDiffRules)
	return auditdiff.Marshal(items)
}

func normalizeSensitiveFieldList(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))
	for _, item := range input {
		trimmed := strings.ToLower(strings.TrimSpace(item))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func loadMaskConfigFromSysConfig() (sensitive.SensitiveFieldsConfig, error) {
	item, err := sys_config.NewSysConfigService().Value(sys_config.AuditSensitiveFieldsConfigKey)
	if err != nil {
		return sensitive.SensitiveFieldsConfig{}, err
	}
	if model.NormalizeValueType(item.ValueType) != model.SysConfigValueTypeJSON {
		return sensitive.SensitiveFieldsConfig{}, fmt.Errorf("%s value_type must be json", sys_config.AuditSensitiveFieldsConfigKey)
	}
	return decodeMaskConfig(item.ConfigValue)
}

func decodeMaskConfig(raw string) (sensitive.SensitiveFieldsConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sensitive.DefaultSensitiveFieldsConfig(), nil
	}
	var config sensitive.SensitiveFieldsConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return sensitive.SensitiveFieldsConfig{}, err
	}
	return sensitive.NormalizeSensitiveFieldsConfig(sensitive.SensitiveFieldsConfig{
		Common:         normalizeSensitiveFieldList(config.Common),
		RequestHeader:  normalizeSensitiveFieldList(config.RequestHeader),
		RequestBody:    normalizeSensitiveFieldList(config.RequestBody),
		ResponseHeader: normalizeSensitiveFieldList(config.ResponseHeader),
		ResponseBody:   normalizeSensitiveFieldList(config.ResponseBody),
	}), nil
}

func saveMaskConfigToSysConfig(config sensitive.SensitiveFieldsConfig) (bool, error) {
	db, err := model.GetDB()
	if err != nil {
		return false, err
	}

	configModel := model.NewSysConfig()
	if !db.Migrator().HasTable(configModel.TableName()) {
		return false, nil
	}

	payload, err := json.Marshal(config)
	if err != nil {
		return false, err
	}

	err = configModel.FindByKey(sys_config.AuditSensitiveFieldsConfigKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		configModel = model.NewSysConfig()
		configModel.ConfigKey = sys_config.AuditSensitiveFieldsConfigKey
		configModel.Sort = requestLogMaskConfigSort
	}

	configModel.ConfigValue = string(payload)
	configModel.ValueType = model.SysConfigValueTypeJSON
	configModel.GroupCode = requestLogMaskConfigGroupCode
	configModel.IsSystem = 1
	configModel.IsSensitive = 1
	configModel.IsVisible = 0
	configModel.ManageTab = requestLogMaskConfigManageTab
	configModel.Status = 1
	configModel.Remark = requestLogMaskConfigRemark
	if configModel.Sort == 0 {
		configModel.Sort = requestLogMaskConfigSort
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		configModel.SetDB(tx)
		if err := configModel.Save(); err != nil {
			return err
		}
		return model.NewSysConfigI18n().UpsertConfigNames(configModel.ID, requestLogMaskConfigNameI18n, tx)
	}); err != nil {
		return false, err
	}
	return true, nil
}

// GenerateExportCSV 生成请求日志 CSV 数据（供异步导出任务使用）。
func GenerateExportCSV(params *form.RequestLogExport) ([]byte, string, error) {
	return generateRequestLogExportCSVBytes(params, nil)
}

// GenerateExportCSVWithProgress 生成请求日志导出 CSV，并在每批处理完成后回调真实进度。
//
// Deprecated: 优先使用 StreamExportCSV 以避免在内存中持有完整 CSV，
// 当前函数仅作为薄包装保留以兼容旧调用方。
func GenerateExportCSVWithProgress(params *form.RequestLogExport, reporter func(processed, total int) error) ([]byte, string, error) {
	return generateRequestLogExportCSVBytes(params, reporter)
}

// StreamExportCSV 流式生成请求日志 CSV，按 batch (requestLogExportBatchSize) 通过 Rows()/ScanRows
// 写入 w，每写完一批回调一次 reporter，返回生成的文件名以及流式过程中遇到的首个错误。
//
// 与 GenerateExportCSVWithProgress 相比，本函数不会在内存中累积全部 CSV，
// 内存峰值取决于 batch 大小而不是总行数，适合搭配 io.Pipe 与 CreateFromReader 串接。
func StreamExportCSV(ctx context.Context, params *form.RequestLogExport, w io.Writer, reporter func(processed, total int) error) (string, error) {
	if w == nil {
		return "", fmt.Errorf("StreamExportCSV: writer is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	query := buildRequestLogQuery(requestLogQueryInput{
		OperatorID:         params.OperatorID,
		OperatorAccount:    params.OperatorAccount,
		OperationStatus:    params.OperationStatus,
		IsHighRisk:         params.IsHighRisk,
		RequestKind:        params.RequestKind,
		ExecutionTimeScope: params.ExecutionTimeScope,
		Method:             params.Method,
		BaseURL:            params.BaseURL,
		OperationName:      params.OperationName,
		IP:                 params.IP,
		StartTime:          params.StartTime,
		EndTime:            params.EndTime,
	})
	condition, args := query.Build()

	limit := params.Limit
	if limit <= 0 {
		limit = defaultRequestLogExportLimit
	}

	db, err := model.GetDB()
	if err != nil {
		return "", err
	}
	db = db.WithContext(ctx)

	baseQuery := db.Model(&model.RequestLogs{})
	if condition != "" {
		baseQuery = baseQuery.Where(condition, args...)
	}
	var totalRows int64
	if err := baseQuery.Count(&totalRows).Error; err != nil {
		return "", err
	}
	total := int(totalRows)
	if total > limit {
		total = limit
	}

	writer := csv.NewWriter(w)
	if err := writer.Write([]string{
		"id",
		"request_id",
		"operator_account",
		"operator_name",
		"ip",
		"request_kind",
		"execution_time_scope",
		"method",
		"base_url",
		"operation_name",
		"operation_status",
		"is_high_risk",
		"response_status",
		"execution_time_ms",
		"execution_time_unit",
		"change_diff",
		"created_at",
	}); err != nil {
		return "", err
	}

	if reporter != nil {
		if err := reporter(0, total); err != nil {
			return "", err
		}
	}

	processed := 0
	for processed < total {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		batchLimit := requestLogExportBatchSize
		if remaining := total - processed; remaining < batchLimit {
			batchLimit = remaining
		}

		queryDB := db.Model(&model.RequestLogs{}).
			Select([]string{
				"id",
				"request_id",
				"operator_account",
				"operator_name",
				"ip",
				"request_kind",
				"execution_time_scope",
				"method",
				"base_url",
				"operation_name",
				"operation_status",
				"is_high_risk",
				"response_status",
				"execution_time",
				"execution_time_unit",
				"change_diff",
				"created_at",
			}).
			Order("created_at DESC, id DESC").
			Limit(batchLimit).
			Offset(processed)
		if condition != "" {
			queryDB = queryDB.Where(condition, args...)
		}

		rows, err := queryDB.Rows()
		if err != nil {
			return "", err
		}

		batchCount, scanErr := writeRequestLogRows(db, rows, writer)
		// 立即关闭 rows，释放连接
		_ = rows.Close()
		if scanErr != nil {
			return "", scanErr
		}
		// 每批 flush 一次，让下游 reader 能尽快拿到数据，避免大批写入造成的卡顿
		writer.Flush()
		if err := writer.Error(); err != nil {
			return "", err
		}
		if batchCount == 0 {
			break
		}
		processed += batchCount
		if reporter != nil {
			if err := reporter(processed, total); err != nil {
				return "", err
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	fileName := "request_logs_" + time.Now().Format("20060102150405") + ".csv"
	return fileName, nil
}

// writeRequestLogRows 流式扫描 rows 并将每一行写入 csv writer，返回写入行数及首个错误。
func writeRequestLogRows(db *gorm.DB, rows *sql.Rows, writer *csv.Writer) (int, error) {
	count := 0
	for rows.Next() {
		record := &model.RequestLogs{}
		if err := db.ScanRows(rows, record); err != nil {
			return count, err
		}
		if err := writer.Write([]string{
			strconv.FormatUint(uint64(record.ID), 10),
			record.RequestID,
			record.OperatorAccount,
			record.OperatorName,
			record.IP,
			record.RequestKind,
			record.ExecutionTimeScope,
			record.Method,
			record.BaseURL,
			record.OperationName,
			strconv.Itoa(record.OperationStatus),
			strconv.Itoa(int(record.IsHighRisk)),
			strconv.Itoa(record.ResponseStatus),
			strconv.FormatFloat(record.ExecutionTime, 'f', 4, 64),
			record.ExecutionTimeUnit,
			record.ChangeDiff,
			record.CreatedAt.String(),
		}); err != nil {
			return count, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	return count, nil
}

// generateRequestLogExportCSVBytes 是 StreamExportCSV 的薄包装：
// 复用同一份流式实现，但把 writer 替换为 bytes.Buffer，便于继续兼容
// 那些直接需要 []byte 形式的旧调用方（例如同步导出接口）。
func generateRequestLogExportCSVBytes(params *form.RequestLogExport, reporter exportProgressReporter) ([]byte, string, error) {
	buffer := &bytes.Buffer{}
	fileName, err := StreamExportCSV(context.Background(), params, buffer, reporter)
	if err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), fileName, nil
}
