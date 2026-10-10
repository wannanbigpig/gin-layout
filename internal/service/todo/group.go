package todo

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func (s *Service) ListGroups(ctx context.Context, userID uint) ([]*model.UserTodoGroup, error) {
	if userID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]*model.UserTodoGroup, 0)
	err = db.Where("user_id = ?", userID).Order("id ASC").Find(&groups).Error
	return groups, err
}

func (s *Service) CreateGroup(ctx context.Context, userID uint, name string) (*model.UserTodoGroup, error) {
	if userID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	name, err := validTodoGroupName(name)
	if err != nil {
		return nil, err
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	group := &model.UserTodoGroup{UserID: userID, Name: name}
	if err := db.Create(group).Error; err != nil {
		if isTodoDuplicate(err) {
			return nil, e.NewBusinessError(e.InvalidParameter, "分组名称已存在")
		}
		return nil, err
	}
	return group, nil
}

func (s *Service) RenameGroup(ctx context.Context, userID, id uint, name string) (*model.UserTodoGroup, error) {
	if userID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	name, err := validTodoGroupName(name)
	if err != nil {
		return nil, err
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	group := model.NewUserTodoGroup()
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := lockOwnedTodoGroup(tx, userID, id, group); err != nil {
			return err
		}
		if group.Name == name {
			return nil
		}
		if err := tx.Model(group).Update("name", name).Error; err != nil {
			if isTodoDuplicate(err) {
				return e.NewBusinessError(e.InvalidParameter, "分组名称已存在")
			}
			return err
		}
		group.Name = name
		return nil
	})
	return group, err
}

func (s *Service) DeleteGroup(ctx context.Context, userID, id uint) error {
	if userID == 0 {
		return e.NewBusinessError(e.NotLogin)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		group := model.NewUserTodoGroup()
		if err := lockOwnedTodoGroup(tx, userID, id, group); err != nil {
			return err
		}
		if err := tx.Unscoped().Model(&model.UserTodo{}).Where("assignee_id = ? AND group_id = ?", userID, id).
			Update("group_id", 0).Error; err != nil {
			return err
		}
		return tx.Delete(group).Error
	})
}

func (s *Service) MoveToGroup(ctx context.Context, userID, todoID, groupID uint) (*model.UserTodo, error) {
	if userID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	var item *model.UserTodo
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := validateOwnedTodoGroup(tx, userID, groupID); err != nil {
			return err
		}
		item, err = loadOwnedTodoForUpdate(tx, userID, todoID)
		if err != nil {
			return err
		}
		if item.GroupID == groupID {
			return nil
		}
		if err := tx.Model(item).Update("group_id", groupID).Error; err != nil {
			return err
		}
		item.GroupID = groupID
		return nil
	})
	return item, err
}

func validTodoGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 40 {
		return "", e.NewBusinessError(e.InvalidParameter, "分组名称应为 1-40 个字符")
	}
	return name, nil
}

func validateOwnedTodoGroup(tx *gorm.DB, userID, groupID uint) error {
	if groupID == 0 {
		return nil
	}
	return lockOwnedTodoGroup(tx, userID, groupID, model.NewUserTodoGroup())
}

func lockOwnedTodoGroup(tx *gorm.DB, userID, groupID uint, group *model.UserTodoGroup) error {
	if groupID == 0 {
		return e.NewBusinessError(e.InvalidParameter)
	}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", groupID, userID).First(group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return e.NewBusinessError(e.NotFound)
	}
	return err
}
