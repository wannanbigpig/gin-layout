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
	query := newLogListQuery().
		addEq("operator_id", uintFilterValue(params.OperatorID)).
		addLike("operator_account", params.OperatorAccount).
		addEq("base_url", params.BaseURL).
		addEq("method", params.Method).
		addLike("operation_name", params.OperationName).
		addLike("ip", params.IP).
		addCreatedAtRange(params.StartTime, params.EndTime)

	if params.OperationStatus != nil {
		switch *params.OperationStatus {
		case 0:
			query.addCondition("operation_status = ?", 0)
		case 1:
			query.addCondition("operation_status != ?", 0)
		}
	}
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
		return nil, e.NewBusinessError(1, "请求日志不存在")
	}
	// 使用资源类转换，详情包含所有字段
	transformer := resources.NewRequestLogTransformer()
	return transformer.ToStruct(requestLog), nil
}
