package model

// UserTodoGroup 是用户私有的待办分组。
type UserTodoGroup struct {
	BaseModel
	UserID uint   `json:"user_id" gorm:"column:user_id;not null"`
	Name   string `json:"name" gorm:"column:name;type:varchar(40);not null"`
}

func NewUserTodoGroup() *UserTodoGroup {
	return BindModel(&UserTodoGroup{})
}

func (m *UserTodoGroup) TableName() string {
	return "user_todo_groups"
}
