package form

// UserTodoList 表示个人待办列表查询。
type UserTodoList struct {
	Paginate
	Status     string `form:"status" json:"status" binding:"omitempty,oneof=pending processing completed canceled"`
	Priority   string `form:"priority" json:"priority" binding:"omitempty,oneof=low normal high urgent"`
	SourceType string `form:"source_type" json:"source_type" binding:"omitempty,max=40"`
	DueScope   string `form:"due_scope" json:"due_scope" binding:"omitempty,oneof=overdue today upcoming no_due"`
	Keyword    string `form:"keyword" json:"keyword" binding:"omitempty,max=100"`
}

func NewUserTodoListQuery() *UserTodoList {
	return &UserTodoList{}
}

// UserTodoCreate 创建当前用户的手工待办。source_type 固定为 manual。
type UserTodoCreate struct {
	SourceKey   string `form:"source_key" json:"source_key" binding:"omitempty,max=128"`
	Title       string `form:"title" json:"title" binding:"required,max=255"`
	Description string `form:"description" json:"description" binding:"omitempty,max=5000"`
	Priority    string `form:"priority" json:"priority" binding:"omitempty,oneof=low normal high urgent"`
	ActionURL   string `form:"action_url" json:"action_url" binding:"omitempty,max=500"`
	DueAt       string `form:"due_at" json:"due_at" binding:"omitempty,max=35"`
}

func NewUserTodoCreateForm() *UserTodoCreate {
	return &UserTodoCreate{}
}

// UserTodoUpdate 只用于编辑当前用户自己的 manual 待办核心信息。
type UserTodoUpdate struct {
	ID          uint    `form:"id" json:"id" binding:"required,gt=0"`
	Title       *string `form:"title" json:"title" binding:"omitempty,min=1,max=255"`
	Description *string `form:"description" json:"description" binding:"omitempty,max=5000"`
	Priority    *string `form:"priority" json:"priority" binding:"omitempty,oneof=low normal high urgent"`
	ActionURL   *string `form:"action_url" json:"action_url" binding:"omitempty,max=500"`
	DueAt       *string `form:"due_at" json:"due_at" binding:"omitempty,max=35"`
}

func NewUserTodoUpdateForm() *UserTodoUpdate {
	return &UserTodoUpdate{}
}

type UserTodoStatus struct {
	ID     uint   `form:"id" json:"id" binding:"required,gt=0"`
	Status string `form:"status" json:"status" binding:"required,oneof=pending processing completed canceled"`
}

func NewUserTodoStatusForm() *UserTodoStatus {
	return &UserTodoStatus{}
}

type UserTodoDelete struct {
	ID uint `form:"id" json:"id" binding:"required,gt=0"`
}

func NewUserTodoDeleteForm() *UserTodoDelete {
	return &UserTodoDelete{}
}
