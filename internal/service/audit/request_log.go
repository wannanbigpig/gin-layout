package audit

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

// RequestLogService 请求日志服务
type RequestLogService struct {
	service.Base
}

// NewRequestLogService 创建请求日志服务实例
func NewRequestLogService() *RequestLogService {
	return &RequestLogService{}
}

// List 分页查询请求日志列表
func (s *RequestLogService) List(params *form.RequestLogList) *resources.Collection {
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
	conditionStr, args := query.Build()
	requestLogModel := model.NewRequestLogs()

	// 构建查询参数，只查询列表需要的字段，排除大字段
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"request_id",
			"operator_id",
			"ip",
			"method",
			"base_url",
			"operation_name",
			"operation_status",
			"is_high_risk",
			"operator_account",
			"operator_name",
			"response_status",
			"execution_time",
			"created_at",
		},
		OrderBy: "created_at DESC, id DESC",
	}
	transformer := resources.NewRequestLogTransformer()

	// 分页查询（只查询列表需要的字段）
	total, collection, err := model.ListPageE(requestLogModel, params.Page, params.PerPage, conditionStr, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询请求日志列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}

	// 使用资源类转换，列表不包含大字段
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// Detail 获取请求日志详情
func (s *RequestLogService) Detail(id uint) (any, error) {
	requestLog := model.NewRequestLogs()
	if err := requestLog.GetById(id); err != nil || requestLog.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	// 使用资源类转换，详情包含所有字段
	transformer := resources.NewRequestLogTransformer()
	return transformer.ToStruct(requestLog), nil
}

type requestLogQueryInput struct {
	OperatorID      uint
	OperatorAccount string
	OperationStatus *int
	IsHighRisk      *uint8
	Method          string
	BaseURL         string
	OperationName   string
	IP              string
	StartTime       string
	EndTime         string
}

func buildRequestLogQuery(input requestLogQueryInput) *logListQuery {
	query := newLogListQuery().
		addEq("operator_id", uintFilterValue(input.OperatorID)).
		addLike("operator_account", input.OperatorAccount).
		addEq("base_url", input.BaseURL).
		addEq("method", input.Method).
		addLike("operation_name", input.OperationName).
		addLike("ip", input.IP).
		addEq("is_high_risk", input.IsHighRisk).
		addCreatedAtRange(input.StartTime, input.EndTime)

	if input.OperationStatus != nil {
		switch *input.OperationStatus {
		case 0:
			query.addCondition("operation_status = ?", 0)
		case 1:
			query.addCondition("operation_status != ?", 0)
		}
	}
	return query
}
