package form

// RequestLogList 请求日志列表查询表单
type RequestLogList struct {
	Paginate
	OperatorID      uint   `form:"operator_id" json:"operator_id" binding:"omitempty"`                                    // 操作ID（用户ID）
	OperatorAccount string `form:"operator_account" json:"operator_account" binding:"omitempty"`                          // 操作账号
	OperationStatus *int   `form:"operation_status" json:"operation_status" binding:"omitempty,oneof=0 1"`                // 操作状态：0=成功，1=失败
	IsHighRisk      *uint8 `form:"is_high_risk" json:"is_high_risk" binding:"omitempty,oneof=0 1"`                        // 是否高危操作
	Method          string `form:"method" json:"method" binding:"omitempty,oneof=GET POST PUT DELETE OPTIONS HEAD PATCH"` // HTTP请求方法
	BaseURL         string `form:"base_url" json:"base_url" binding:"omitempty"`                                          // 请求基础URL
	OperationName   string `form:"operation_name" json:"operation_name" binding:"omitempty"`                              // 操作接口
	IP              string `form:"ip" json:"ip" binding:"omitempty"`                                                      // 操作IP
	StartTime       string `form:"start_time" json:"start_time" binding:"omitempty"`                                      // 开始时间
	EndTime         string `form:"end_time" json:"end_time" binding:"omitempty"`                                          // 结束时间
}

// NewRequestLogListQuery 创建请求日志列表查询表单
func NewRequestLogListQuery() *RequestLogList {
	return &RequestLogList{}
}

// RequestLogExport 请求日志导出查询参数。
type RequestLogExport struct {
	OperatorID      uint   `form:"operator_id" json:"operator_id" binding:"omitempty"`
	OperatorAccount string `form:"operator_account" json:"operator_account" binding:"omitempty"`
	OperationStatus *int   `form:"operation_status" json:"operation_status" binding:"omitempty,oneof=0 1"`
	IsHighRisk      *uint8 `form:"is_high_risk" json:"is_high_risk" binding:"omitempty,oneof=0 1"`
	Method          string `form:"method" json:"method" binding:"omitempty,oneof=GET POST PUT DELETE OPTIONS HEAD PATCH"`
	BaseURL         string `form:"base_url" json:"base_url" binding:"omitempty"`
	OperationName   string `form:"operation_name" json:"operation_name" binding:"omitempty"`
	IP              string `form:"ip" json:"ip" binding:"omitempty"`
	StartTime       string `form:"start_time" json:"start_time" binding:"omitempty"`
	EndTime         string `form:"end_time" json:"end_time" binding:"omitempty"`
	Limit           int    `form:"limit" json:"limit" binding:"omitempty,min=1,max=5000"`
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
