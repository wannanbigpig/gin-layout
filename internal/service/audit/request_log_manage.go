package audit

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
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

const defaultRequestLogExportLimit = 1000

const (
	requestLogMaskConfigGroupCode = "audit"
	requestLogMaskConfigSort      = 95
	requestLogMaskConfigRemark    = "请求日志脱敏字段配置"
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
	query := buildRequestLogQuery(requestLogQueryInput{
		OperatorID:      params.OperatorID,
		OperatorAccount: params.OperatorAccount,
		OperationStatus: params.OperationStatus,
		IsHighRisk:      params.IsHighRisk,
		Method:          params.Method,
		BaseURL:         params.BaseURL,
		OperationName:   params.OperationName,
		IP:              params.IP,
		StartTime:       params.StartTime,
		EndTime:         params.EndTime,
	})
	condition, args := query.Build()

	limit := params.Limit
	if limit <= 0 {
		limit = defaultRequestLogExportLimit
	}

	logModel := model.NewRequestLogs()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"request_id",
			"operator_account",
			"operator_name",
			"ip",
			"method",
			"base_url",
			"operation_name",
			"operation_status",
			"is_high_risk",
			"response_status",
			"execution_time",
			"change_diff",
			"created_at",
		},
		OrderBy: "created_at DESC, id DESC",
	}
	_, records, err := model.ListPageE(logModel, 1, limit, condition, args, listOptionalParams)
	if err != nil {
		return nil, "", err
	}

	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write([]string{
		"id",
		"request_id",
		"operator_account",
		"operator_name",
		"ip",
		"method",
		"base_url",
		"operation_name",
		"operation_status",
		"is_high_risk",
		"response_status",
		"execution_time_ms",
		"change_diff",
		"created_at",
	}); err != nil {
		return nil, "", err
	}

	for _, record := range records {
		if err := writer.Write([]string{
			strconv.FormatUint(uint64(record.ID), 10),
			record.RequestID,
			record.OperatorAccount,
			record.OperatorName,
			record.IP,
			record.Method,
			record.BaseURL,
			record.OperationName,
			strconv.Itoa(record.OperationStatus),
			strconv.Itoa(int(record.IsHighRisk)),
			strconv.Itoa(record.ResponseStatus),
			strconv.FormatFloat(record.ExecutionTime, 'f', 4, 64),
			record.ChangeDiff,
			record.CreatedAt.String(),
		}); err != nil {
			return nil, "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", err
	}

	fileName := "request_logs_" + time.Now().Format("20060102150405") + ".csv"
	return buffer.Bytes(), fileName, nil
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
	return sensitive.SensitiveFieldsConfig{
		Common:         normalizeSensitiveFieldList(config.Common),
		RequestHeader:  normalizeSensitiveFieldList(config.RequestHeader),
		RequestBody:    normalizeSensitiveFieldList(config.RequestBody),
		ResponseHeader: normalizeSensitiveFieldList(config.ResponseHeader),
		ResponseBody:   normalizeSensitiveFieldList(config.ResponseBody),
	}, nil
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
