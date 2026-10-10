package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// UserTodoResources 表示个人待办响应。
type UserTodoResources struct {
	ID              uint              `json:"id"`
	AssigneeID      uint              `json:"assignee_id"`
	CreatorID       uint              `json:"creator_id"`
	GroupID         uint              `json:"group_id"`
	SourceType      string            `json:"source_type"`
	SourceKey       string            `json:"source_key"`
	Title           string            `json:"title"`
	Description     string            `json:"description"`
	Priority        string            `json:"priority"`
	Status          string            `json:"status"`
	ActionURL       string            `json:"action_url"`
	DueAt           *utils.FormatDate `json:"due_at"`
	CompletedAt     *utils.FormatDate `json:"completed_at"`
	CanceledAt      *utils.FormatDate `json:"canceled_at"`
	CanEdit         bool              `json:"can_edit"`
	CanDelete       bool              `json:"can_delete"`
	CanUpdateStatus bool              `json:"can_update_status"`
	CreatedAt       utils.FormatDate  `json:"created_at"`
	UpdatedAt       utils.FormatDate  `json:"updated_at"`
}

type UserTodoSummaryResources struct {
	Total      int64 `json:"total"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Canceled   int64 `json:"canceled"`
	Overdue    int64 `json:"overdue"`
	DueToday   int64 `json:"due_today"`
}

// UserTodoListResources 在分页数据中附带当前分组的全量状态统计。
type UserTodoListResources struct {
	*Collection
	Summary *UserTodoSummaryResources `json:"summary"`
}

type UserTodoGroupResources struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func NewUserTodoGroupResource(group *model.UserTodoGroup) *UserTodoGroupResources {
	if group == nil {
		return nil
	}
	return &UserTodoGroupResources{ID: group.ID, Name: group.Name}
}

func NewUserTodoGroupCollection(groups []*model.UserTodoGroup) []UserTodoGroupResources {
	result := make([]UserTodoGroupResources, 0, len(groups))
	for _, group := range groups {
		result = append(result, *NewUserTodoGroupResource(group))
	}
	return result
}

type UserTodoTransformer struct{}

func NewUserTodoTransformer() UserTodoTransformer {
	return UserTodoTransformer{}
}

func (UserTodoTransformer) ToStruct(item *model.UserTodo) *UserTodoResources {
	if item == nil {
		return nil
	}
	manual := item.SourceType == model.TodoSourceManual
	return &UserTodoResources{
		ID:              item.ID,
		AssigneeID:      item.AssigneeID,
		CreatorID:       item.CreatorID,
		GroupID:         item.GroupID,
		SourceType:      item.SourceType,
		SourceKey:       item.SourceKey,
		Title:           item.Title,
		Description:     item.Description,
		Priority:        item.Priority,
		Status:          item.Status,
		ActionURL:       item.ActionURL,
		DueAt:           item.DueAt,
		CompletedAt:     item.CompletedAt,
		CanceledAt:      item.CanceledAt,
		CanEdit:         manual,
		CanDelete:       manual,
		CanUpdateStatus: manual,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func (r UserTodoTransformer) ToCollection(page, perPage int, total int64, data []*model.UserTodo) *Collection {
	items := make([]any, 0, len(data))
	for _, item := range data {
		items = append(items, r.ToStruct(item))
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(items)
}
