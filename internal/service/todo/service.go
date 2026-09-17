package todo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"uuid"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type Service struct {
	db  *gorm.DB
	now func() time.Time
}

type BusinessTodoInput struct {
	AssigneeID uint
	CreatorID  uint
	SourceType string
	SourceKey  string
	// StartNewCycle 仅由来源模块在业务对象重新进入待处理状态时设置。
	// 普通重复投递保持 false，避免把已经关闭的待办重新打开。
	StartNewCycle bool
	Title         string
	Description   string
	Priority      string
	ActionURL     string
	DueAt         *time.Time
}

type BusinessTodoCloseInput struct {
	AssigneeID uint
	SourceType string
	SourceKey  string
	Status     string
}

func NewService() *Service {
	return &Service{now: time.Now}
}

func NewServiceWithDB(db *gorm.DB) *Service {
	return &Service{db: db, now: time.Now}
}

func (s *Service) dbOrDefault(ctx context.Context) (*gorm.DB, error) {
	db := s.db
	if db == nil {
		var err error
		db, err = model.GetDB()
		if err != nil {
			return nil, err
		}
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	return db, nil
}

func (s *Service) currentTime() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

func (s *Service) List(ctx context.Context, assigneeID uint, params *form.UserTodoList) (*resources.Collection, error) {
	if assigneeID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	if params == nil {
		params = form.NewUserTodoListQuery()
	}
	page, perPage := params.Page, params.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	query := db.Model(&model.UserTodo{}).Where("assignee_id = ?", assigneeID)
	if status := strings.TrimSpace(params.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if priority := strings.TrimSpace(params.Priority); priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if sourceType := strings.TrimSpace(params.SourceType); sourceType != "" {
		query = query.Where("source_type = ?", sourceType)
	}
	if keyword := strings.TrimSpace(params.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(title LIKE ? OR description LIKE ?)", like, like)
	}
	query = applyDueScope(query, strings.TrimSpace(params.DueScope), s.currentTime())

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	items := make([]*model.UserTodo, 0)
	if err := query.
		Clauses(clause.OrderBy{Expression: clause.Expr{
			SQL:  "CASE WHEN status IN (?, ?) THEN 0 ELSE 1 END ASC, CASE WHEN status IN (?, ?) AND due_at IS NOT NULL AND due_at < ? THEN 0 ELSE 1 END ASC, CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END ASC, CASE WHEN due_at IS NULL THEN 1 ELSE 0 END ASC, due_at ASC, created_at DESC, id DESC",
			Vars: []any{model.TodoStatusPending, model.TodoStatusProcessing, model.TodoStatusPending, model.TodoStatusProcessing, s.currentTime()},
		}}).
		Offset((page - 1) * perPage).Limit(perPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return resources.NewUserTodoTransformer().ToCollection(page, perPage, total, items), nil
}

func applyDueScope(db *gorm.DB, scope string, now time.Time) *gorm.DB {
	active := []string{model.TodoStatusPending, model.TodoStatusProcessing}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	next := start.AddDate(0, 0, 1)
	switch scope {
	case "overdue":
		return db.Where("status IN ? AND due_at IS NOT NULL AND due_at < ?", active, now)
	case "today":
		return db.Where("status IN ? AND due_at >= ? AND due_at < ?", active, start, next)
	case "upcoming":
		return db.Where("status IN ? AND due_at >= ?", active, next)
	case "no_due":
		return db.Where("due_at IS NULL")
	default:
		return db
	}
}

func (s *Service) Summary(ctx context.Context, assigneeID uint) (*resources.UserTodoSummaryResources, error) {
	if assigneeID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	now := s.currentTime()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	next := start.AddDate(0, 0, 1)
	var row struct {
		Pending    int64
		Processing int64
		Overdue    int64
		DueToday   int64
	}
	err = db.Model(&model.UserTodo{}).
		Select(`COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS pending,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS processing,
			COALESCE(SUM(CASE WHEN status IN ? AND due_at IS NOT NULL AND due_at < ? THEN 1 ELSE 0 END), 0) AS overdue,
			COALESCE(SUM(CASE WHEN status IN ? AND due_at >= ? AND due_at < ? THEN 1 ELSE 0 END), 0) AS due_today`,
			model.TodoStatusPending, model.TodoStatusProcessing,
			[]string{model.TodoStatusPending, model.TodoStatusProcessing}, now,
			[]string{model.TodoStatusPending, model.TodoStatusProcessing}, start, next).
		Where("assignee_id = ?", assigneeID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &resources.UserTodoSummaryResources{
		Pending: row.Pending, Processing: row.Processing, Overdue: row.Overdue, DueToday: row.DueToday,
	}, nil
}

func (s *Service) CreateManual(ctx context.Context, assigneeID uint, params *form.UserTodoCreate) (*model.UserTodo, error) {
	if assigneeID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	if params == nil || strings.TrimSpace(params.Title) == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "待办标题不能为空")
	}
	dueAt, err := parseDueAt(params.DueAt)
	if err != nil {
		return nil, err
	}
	actionURL, err := normalizeActionURL(params.ActionURL)
	if err != nil {
		return nil, err
	}
	sourceKey := strings.TrimSpace(params.SourceKey)
	if sourceKey == "" {
		sourceKey = "manual:" + uuid.New().String()
	}
	item := model.NewUserTodo()
	item.AssigneeID = assigneeID
	item.CreatorID = assigneeID
	item.SourceType = model.TodoSourceManual
	item.SourceKey = sourceKey
	item.Title = strings.TrimSpace(params.Title)
	item.Description = strings.TrimSpace(params.Description)
	item.Priority = normalizePriority(params.Priority)
	item.Status = model.TodoStatusPending
	item.ActionURL = actionURL
	item.DueAt = dueAt
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if err := db.Create(item).Error; err != nil {
		if isTodoDuplicate(err) {
			return nil, e.NewBusinessError(e.InvalidParameter, "待办标识已存在")
		}
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateManual(ctx context.Context, assigneeID uint, params *form.UserTodoUpdate) (*model.UserTodo, error) {
	if assigneeID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	if params == nil || params.ID == 0 {
		return nil, e.NewBusinessError(e.InvalidParameter)
	}
	updates := make(map[string]any)
	if params.Title != nil {
		title := strings.TrimSpace(*params.Title)
		if title == "" {
			return nil, e.NewBusinessError(e.InvalidParameter, "待办标题不能为空")
		}
		updates["title"] = title
	}
	if params.Description != nil {
		updates["description"] = strings.TrimSpace(*params.Description)
	}
	if params.Priority != nil {
		if !validPriority(*params.Priority) {
			return nil, e.NewBusinessError(e.InvalidParameter, "待办优先级无效")
		}
		updates["priority"] = strings.TrimSpace(*params.Priority)
	}
	if params.ActionURL != nil {
		actionURL, err := normalizeActionURL(*params.ActionURL)
		if err != nil {
			return nil, err
		}
		updates["action_url"] = actionURL
	}
	if params.DueAt != nil {
		dueAt, err := parseDueAt(*params.DueAt)
		if err != nil {
			return nil, err
		}
		updates["due_at"] = dueAt
	}
	if len(updates) == 0 {
		return nil, e.NewBusinessError(e.InvalidParameter, "没有可更新的待办字段")
	}
	var updated *model.UserTodo
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		item, err := loadOwnedTodoForUpdate(tx, assigneeID, params.ID)
		if err != nil {
			return err
		}
		if item.SourceType != model.TodoSourceManual {
			return e.NewBusinessError(e.InvalidParameter, "业务来源待办不允许通过个人接口编辑")
		}
		if err := tx.Model(item).Updates(updates).Error; err != nil {
			return err
		}
		updated = item
		return tx.First(updated, item.ID).Error
	})
	return updated, err
}

func (s *Service) UpdateStatus(ctx context.Context, assigneeID, id uint, status string) (*model.UserTodo, error) {
	if assigneeID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	status = strings.TrimSpace(status)
	if !validStatus(status) {
		return nil, e.NewBusinessError(e.InvalidParameter, "待办状态无效")
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	var updated *model.UserTodo
	err = db.Transaction(func(tx *gorm.DB) error {
		item, err := loadOwnedTodoForUpdate(tx, assigneeID, id)
		if err != nil {
			return err
		}
		if item.SourceType != model.TodoSourceManual {
			return e.NewBusinessError(e.InvalidParameter, "业务来源待办需在对应业务中处理")
		}
		if item.Status == status {
			updated = item
			return nil
		}
		if !canTransitionTodoStatus(item.Status, status) {
			return e.NewBusinessError(e.InvalidParameter, fmt.Sprintf("待办状态不允许从 %s 变更为 %s", item.Status, status))
		}
		updates := todoStatusUpdates(status, s.currentTime())
		if err := tx.Model(item).Updates(updates).Error; err != nil {
			return err
		}
		updated = item
		return tx.First(updated, item.ID).Error
	})
	return updated, err
}

func (s *Service) DeleteManual(ctx context.Context, assigneeID, id uint) error {
	if assigneeID == 0 {
		return e.NewBusinessError(e.NotLogin)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		item, err := loadOwnedTodoForUpdate(tx, assigneeID, id)
		if err != nil {
			return err
		}
		if item.SourceType != model.TodoSourceManual {
			return e.NewBusinessError(e.InvalidParameter, "业务来源待办不允许通过个人接口删除")
		}
		return tx.Delete(item).Error
	})
}

// UpsertBusinessTodo 按 assignee/source_type/source_key 幂等创建或更新业务待办。
// 重复同步只更新业务描述字段，保留用户处理状态，避免业务重试重新打开已关闭待办。
func (s *Service) UpsertBusinessTodo(ctx context.Context, input BusinessTodoInput) (*model.UserTodo, error) {
	input.SourceType = strings.ToLower(strings.TrimSpace(input.SourceType))
	input.SourceKey = strings.TrimSpace(input.SourceKey)
	input.Title = strings.TrimSpace(input.Title)
	if input.AssigneeID == 0 || input.SourceType == "" || input.SourceType == model.TodoSourceManual || input.SourceKey == "" || input.Title == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "业务待办参数不完整")
	}
	if len([]rune(input.SourceType)) > 40 || len([]rune(input.SourceKey)) > 128 || len([]rune(input.Title)) > 255 {
		return nil, e.NewBusinessError(e.InvalidParameter, "业务待办字段超出长度限制")
	}
	priority := normalizePriority(input.Priority)
	if !validPriority(priority) {
		return nil, e.NewBusinessError(e.InvalidParameter, "待办优先级无效")
	}
	actionURL, err := normalizeActionURL(input.ActionURL)
	if err != nil {
		return nil, err
	}
	var dueAt *utils.FormatDate
	if input.DueAt != nil {
		dueAt = &utils.FormatDate{Time: input.DueAt.In(time.Local)}
	}
	item := model.NewUserTodo()
	item.AssigneeID = input.AssigneeID
	item.CreatorID = input.CreatorID
	item.SourceType = input.SourceType
	item.SourceKey = input.SourceKey
	item.Title = input.Title
	item.Description = strings.TrimSpace(input.Description)
	item.Priority = priority
	item.Status = model.TodoStatusPending
	item.ActionURL = actionURL
	item.DueAt = dueAt
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	latest := model.NewUserTodo()
	err = db.Where("assignee_id = ? AND source_type = ? AND source_key = ?", input.AssigneeID, input.SourceType, input.SourceKey).
		Order("id DESC").First(latest).Error
	if err == nil && !input.StartNewCycle && (latest.Status == model.TodoStatusCompleted || latest.Status == model.TodoStatusCanceled) {
		return latest, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	err = db.Clauses(clause.OnConflict{
		DoUpdates: clause.Assignments(map[string]any{
			"title": input.Title, "description": item.Description, "priority": priority,
			"action_url": item.ActionURL, "due_at": dueAt, "updated_at": s.currentTime(),
		}),
	}).Create(item).Error
	if err != nil {
		return nil, err
	}
	result := model.NewUserTodo()
	if err := db.Where("assignee_id = ? AND source_type = ? AND source_key = ? AND status IN ?",
		input.AssigneeID, input.SourceType, input.SourceKey,
		[]string{model.TodoStatusPending, model.TodoStatusProcessing}).Order("id DESC").First(result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

// CloseBusinessTodo 幂等关闭一个业务来源待办，仅接受 completed/canceled。
func (s *Service) CloseBusinessTodo(ctx context.Context, input BusinessTodoCloseInput) (*model.UserTodo, error) {
	input.SourceType = strings.ToLower(strings.TrimSpace(input.SourceType))
	input.SourceKey = strings.TrimSpace(input.SourceKey)
	input.Status = strings.TrimSpace(input.Status)
	if input.AssigneeID == 0 || input.SourceType == "" || input.SourceType == model.TodoSourceManual || input.SourceKey == "" ||
		(input.Status != model.TodoStatusCompleted && input.Status != model.TodoStatusCanceled) {
		return nil, e.NewBusinessError(e.InvalidParameter, "业务待办关闭参数无效")
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	var closed *model.UserTodo
	err = db.Transaction(func(tx *gorm.DB) error {
		item := model.NewUserTodo()
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("assignee_id = ? AND source_type = ? AND source_key = ? AND status IN ?",
				input.AssigneeID, input.SourceType, input.SourceKey,
				[]string{model.TodoStatusPending, model.TodoStatusProcessing}).
			Order("id DESC").
			First(item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = tx.Where("assignee_id = ? AND source_type = ? AND source_key = ?", input.AssigneeID, input.SourceType, input.SourceKey).
				Order("id DESC").First(item).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return e.NewBusinessError(e.NotFound)
			}
		}
		if err != nil {
			return err
		}
		if item.Status == input.Status {
			closed = item
			return nil
		}
		if !canTransitionTodoStatus(item.Status, input.Status) {
			return e.NewBusinessError(e.InvalidParameter, "已关闭待办不允许切换终态")
		}
		if err := tx.Model(item).Updates(todoStatusUpdates(input.Status, s.currentTime())).Error; err != nil {
			return err
		}
		closed = item
		return tx.First(closed, item.ID).Error
	})
	return closed, err
}

func loadOwnedTodoForUpdate(tx *gorm.DB, assigneeID, id uint) (*model.UserTodo, error) {
	item := model.NewUserTodo()
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND assignee_id = ?", id, assigneeID).First(item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.NewBusinessError(e.NotFound)
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func parseDueAt(value string) (*utils.FormatDate, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, ok := utils.ParseTimeBoundary(value)
	if !ok {
		return nil, e.NewBusinessError(e.InvalidParameter, "due_at 格式应为 YYYY-MM-DD HH:mm:ss 或 RFC3339")
	}
	return &utils.FormatDate{Time: parsed}, nil
}

func normalizeActionURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, `\`) {
		return "", e.NewBusinessError(e.InvalidParameter, "action_url 仅支持站内相对路径")
	}
	return value, nil
}

func normalizePriority(priority string) string {
	priority = strings.TrimSpace(priority)
	if priority == "" {
		return model.TodoPriorityNormal
	}
	return priority
}

func validPriority(priority string) bool {
	switch strings.TrimSpace(priority) {
	case model.TodoPriorityLow, model.TodoPriorityNormal, model.TodoPriorityHigh, model.TodoPriorityUrgent:
		return true
	default:
		return false
	}
}

func validStatus(status string) bool {
	switch status {
	case model.TodoStatusPending, model.TodoStatusProcessing, model.TodoStatusCompleted, model.TodoStatusCanceled:
		return true
	default:
		return false
	}
}

func canTransitionTodoStatus(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case model.TodoStatusPending:
		return to == model.TodoStatusProcessing || to == model.TodoStatusCompleted || to == model.TodoStatusCanceled
	case model.TodoStatusProcessing:
		return to == model.TodoStatusPending || to == model.TodoStatusCompleted || to == model.TodoStatusCanceled
	default:
		return false
	}
}

func todoStatusUpdates(status string, now time.Time) map[string]any {
	updates := map[string]any{"status": status, "completed_at": nil, "canceled_at": nil}
	timestamp := &utils.FormatDate{Time: now}
	if status == model.TodoStatusCompleted {
		updates["completed_at"] = timestamp
	}
	if status == model.TodoStatusCanceled {
		updates["canceled_at"] = timestamp
	}
	return updates
}

func isTodoDuplicate(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "unique constraint")
}
