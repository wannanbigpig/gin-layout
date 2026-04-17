package admin

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// AdminUserService 授权服务。
type AdminUserService struct {
	service.Base
}

// NewAdminUserService 创建管理员用户服务实例。
func NewAdminUserService() *AdminUserService {
	return &AdminUserService{}
}

func (s *AdminUserService) handleMutationError(err error, fallback string) error {
	if err == nil {
		return nil
	}

	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) {
		return businessErr
	}

	return e.NewBusinessError(e.FAILURE, fallback)
}

func (s *AdminUserService) revokeUserTokens(tx *gorm.DB, userID uint, revokedCode uint8, revokedReason string) {
	loginService := auth.NewLoginService()
	if err := loginService.RevokeUserTokens(userID, revokedCode, revokedReason, tx); err != nil {
		log.Logger.Error("撤销用户token失败", zap.Error(err), zap.Uint("user_id", userID))
	}
}

// GetUserInfo 获取用户信息。
func (s *AdminUserService) GetUserInfo(id uint) (*resources.AdminUserResources, error) {
	adminUsersModel := model.NewAdminUsers()
	err := adminUsersModel.GetByIdWithPreload(id, "RoleList", "Department")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.FAILURE, "用户不存在")
		}
		return nil, err
	}

	return resources.NewAdminUserTransformer().ToStruct(adminUsersModel), nil
}

// GetUserMenuInfo 获取用户权限信息。
func (s *AdminUserService) GetUserMenuInfo(id uint) (any, error) {
	adminUsersModel := model.NewAdminUsers()
	err := adminUsersModel.GetById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.FAILURE, "用户不存在")
		}
		return nil, err
	}
	condition, args := s.userMenuQuery(id == global.SuperAdminId, nil)
	if id != global.SuperAdminId {
		menuIDs, err := access.NewPermissionSyncCoordinator().AccessibleMenuIDs(id, true)
		if err != nil {
			return nil, err
		}
		condition, args = s.userMenuQuery(false, menuIDs)
	}

	menus, err := model.ListE(model.NewMenu(), condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	if err != nil {
		return nil, err
	}

	return resources.NewMenuTreeTransformer().BuildTreeByNode(menus, 0), nil
}

// List 返回管理员分页列表。
func (s *AdminUserService) List(params *form.AdminUserList) *resources.Collection {
	conditionStr, args := s.buildListCondition(params)
	adminUserModel := model.NewAdminUsers()
	total, collection, err := model.ListPageE(adminUserModel, params.Page, params.PerPage, conditionStr, args, s.adminUserListOptions())
	if err != nil {
		log.Logger.Error("查询管理员列表失败", zap.Error(err))
		return resources.NewAdminUserTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return resources.NewAdminUserTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

func (s *AdminUserService) adminUserListOptions() model.ListOptionalParams {
	// 列表页仅使用部门 id/name/pid，避免 Preload 全字段带来额外 IO。
	return model.ListOptionalParams{
		OrderBy: "created_at desc, id desc",
		Preload: map[string]func(db *gorm.DB) *gorm.DB{
			"Department": func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "name", "pid")
			},
		},
	}
}

func (s *AdminUserService) buildListCondition(params *form.AdminUserList) (string, []any) {
	qb := query_builder.New().
		AddLike("username", params.UserName).
		AddEq("id", zeroToNil(params.ID)).
		AddLike("nickname", params.NickName).
		AddLike("email", params.Email).
		AddLike("full_phone_number", params.PhoneNumber).
		AddEq("status", params.Status)

	if params.DeptId > 0 {
		qb.AddCondition(
			"EXISTS (SELECT 1 FROM admin_user_department_map WHERE admin_user_department_map.uid = admin_user.id AND admin_user_department_map.dept_id = ?)",
			params.DeptId,
		)
	}

	return qb.Build()
}

func zeroToNil(value uint) any {
	if value == 0 {
		return nil
	}
	return value
}

func (s *AdminUserService) userMenuQuery(isSuperAdmin bool, menuIDs []uint) (string, []any) {
	if isSuperAdmin {
		return "status = ?", []any{1}
	}
	if len(menuIDs) == 0 {
		return "status = ? AND is_auth = ?", []any{1, 0}
	}
	return "status = ? AND (is_auth = ? OR (is_auth = ? AND id IN (?)))", []any{1, 0, 1, menuIDs}
}

type adminUserEditParams struct {
	Id          uint
	Username    *string
	Nickname    *string
	Password    *string
	PhoneNumber *string
	CountryCode *string
	Email       *string
	Status      *uint8
	Avatar      *string
	DeptIds     *[]uint
}
