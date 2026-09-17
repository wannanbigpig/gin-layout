package form

// RequestLogList 请求日志列表查询表单
type RequestLogList struct {
	HasChangeDiff uint8 `form:"has_change_diff" json:"has_change_diff" binding:"omitempty,oneof=0 1"` // 1=仅有数据变更差异
	Paginate
	Scope              string `form:"scope" json:"scope" binding:"omitempty,oneof=regular all important changes failed high_risk self_reads"` // 个人日志范围
	RequestID          string `form:"request_id" json:"request_id" binding:"omitempty,max=64"`                                                // 请求ID（精确匹配）
	OperatorID         uint   `form:"operator_id" json:"operator_id" binding:"omitempty"`                                                     // 操作ID（用户ID）
	OperatorAccount    string `form:"operator_account" json:"operator_account" binding:"omitempty"`                                           // 操作账号
	OperationStatus    *int   `form:"operation_status" json:"operation_status" binding:"omitempty,oneof=0 1"`                                 // 操作状态筛选：0=成功，1=失败（匹配所有非0状态码）
	IsHighRisk         *uint8 `form:"is_high_risk" json:"is_high_risk" binding:"omitempty,oneof=0 1"`                                         // 是否高危操作
	RequestKind        string `form:"request_kind" json:"request_kind" binding:"omitempty,oneof=http websocket"`                              // 请求类型：http/websocket
	ExecutionTimeScope string `form:"execution_time_scope" json:"execution_time_scope" binding:"omitempty,oneof=request connection none"`     // 执行时间语义
	Method             string `form:"method" json:"method" binding:"omitempty,oneof=GET POST PUT DELETE OPTIONS HEAD PATCH"`                  // HTTP请求方法
	BaseURL            string `form:"base_url" json:"base_url" binding:"omitempty"`                                                           // 请求基础URL
	OperationName      string `form:"operation_name" json:"operation_name" binding:"omitempty"`                                               // 操作接口
	IP                 string `form:"ip" json:"ip" binding:"omitempty"`                                                                       // 操作IP
	StartTime          string `form:"start_time" json:"start_time" binding:"omitempty"`                                                       // 开始时间
	EndTime            string `form:"end_time" json:"end_time" binding:"omitempty"`                                                           // 结束时间
	OrderBy            string `form:"order_by" json:"order_by" binding:"omitempty,max=64"`                                                    // 排序字段，例如 execution_time_us desc
}

// NewRequestLogListQuery 创建请求日志列表查询表单
func NewRequestLogListQuery() *RequestLogList {
	return &RequestLogList{}
}

// RequestLogExport 请求日志导出查询参数。
type RequestLogExport struct {
	HasChangeDiff      uint8  `form:"has_change_diff" json:"has_change_diff" binding:"omitempty,oneof=0 1"`
	RequestID          string `form:"request_id" json:"request_id" binding:"omitempty,max=64"`
	OperatorID         uint   `form:"operator_id" json:"operator_id" binding:"omitempty"`
	OperatorAccount    string `form:"operator_account" json:"operator_account" binding:"omitempty"`
	OperationStatus    *int   `form:"operation_status" json:"operation_status" binding:"omitempty,oneof=0 1"`
	IsHighRisk         *uint8 `form:"is_high_risk" json:"is_high_risk" binding:"omitempty,oneof=0 1"`
	RequestKind        string `form:"request_kind" json:"request_kind" binding:"omitempty,oneof=http websocket"`
	ExecutionTimeScope string `form:"execution_time_scope" json:"execution_time_scope" binding:"omitempty,oneof=request connection none"`
	Method             string `form:"method" json:"method" binding:"omitempty,oneof=GET POST PUT DELETE OPTIONS HEAD PATCH"`
	BaseURL            string `form:"base_url" json:"base_url" binding:"omitempty"`
	OperationName      string `form:"operation_name" json:"operation_name" binding:"omitempty"`
	IP                 string `form:"ip" json:"ip" binding:"omitempty"`
	StartTime          string `form:"start_time" json:"start_time" binding:"omitempty"`
	EndTime            string `form:"end_time" json:"end_time" binding:"omitempty"`
	Limit              int    `form:"limit" json:"limit" binding:"omitempty,min=1,max=5000"`
}

func NewRequestLogExportQuery() *RequestLogExport {
	return &RequestLogExport{Limit: 1000}
}

// RequestLogMaskConfigForm 请求日志脱敏配置。
type RequestLogMaskConfigForm struct {
	Common         []string `form:"common" json:"common" binding:"omitempty,dive,max=64"`
	RequestHeader  []string `form:"request_header" json:"request_header" binding:"omitempty,dive,max=64"`
	RequestBody    []string `form:"request_body" json:"request_body" binding:"omitempty,dive,max=64"`
	ResponseHeader []string `form:"response_header" json:"response_header" binding:"omitempty,dive,max=64"`
	ResponseBody   []string `form:"response_body" json:"response_body" binding:"omitempty,dive,max=64"`
}

func NewRequestLogMaskConfigForm() *RequestLogMaskConfigForm {
	return &RequestLogMaskConfigForm{}
}
