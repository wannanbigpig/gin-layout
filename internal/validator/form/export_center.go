package form

// ExportRecordList 导出记录列表查询参数。
type ExportRecordList struct {
	Paginate
	Scene         string `form:"scene" json:"scene" binding:"omitempty"`
	Status        string `form:"status" json:"status" binding:"omitempty,oneof=pending running success failed canceled retrying"`
	PublicID      string `form:"public_id" json:"public_id" binding:"omitempty,uuid"`
	TaskRunID     *uint  `form:"task_run_id" json:"task_run_id" binding:"omitempty,gt=0"`
	TriggerUserID *uint  `form:"trigger_user_id" json:"trigger_user_id" binding:"omitempty,gt=0"`
}

func NewExportRecordListQuery() *ExportRecordList {
	return &ExportRecordList{}
}

type MyExportRecordDetail struct {
	PublicID string `form:"public_id" binding:"required,uuid"`
}
