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

const (
	menuQuerySuperAdmin = "status = ?"
	menuQueryNoAuth     = "status = ? AND is_auth = ?"
	menuQueryWithAuth   = "status = ? AND (is_auth = ? OR (is_auth = ? AND id IN (?)))"
)

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

// adminUserListOptions 返回管理员列表查询选项。
// 列表页仅使用部门 id/name/pid，避免 Preload 全字段带来额外 IO。
func (s *AdminUserService) adminUserListOptions() model.ListOptionalParams {
	return model.ListOptionalParams{
		OrderBy: "created_at desc, id desc",
		Preload: map[string]func(db *gorm.DB) *gorm.DB{
			"Department": func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "name", "pid")
			},
		},
	}
}

// buildListCondition 构建管理员列表查询条件。
func (s *AdminUserService) buildListCondition(params *form.AdminUserList) (string, []any) {
	qb := query_builder.New().
		AddLike("username", params.UserName).
		AddEq("id", zeroToNil(params.ID)).
		AddLike("nickname", params.NickName).
		AddLike("email", params.Email).
		AddLike("full_phone_number", params.PhoneNumber).
		AddEq("status", params.Status)

	// 部门筛选：使用 EXISTS 子查询关联用户 - 部门映射表
	if params.DeptId > 0 {
		qb.AddCondition(
			"EXISTS (SELECT 1 FROM admin_user_department_map WHERE admin_user_department_map.uid = admin_user.id AND admin_user_department_map.dept_id = ?)",
			params.DeptId,
		)
	}

	return qb.Build()
}

// zeroToNil 将 0 转换为 nil，用于查询条件构建时排除空值筛选。
func zeroToNil(value uint) any {
	if value == 0 {
		return nil
	}
	return value
}

// userMenuQuery 构建用户菜单查询条件。
// 参数：
//   - isSuperAdmin: 是否为超级管理员
//   - menuIDs: 用户可访问的菜单 ID 列表
//
// 返回：查询条件和参数
func (s *AdminUserService) userMenuQuery(isSuperAdmin bool, menuIDs []uint) (string, []any) {
	if isSuperAdmin {
		return menuQuerySuperAdmin, []any{1}
	}
	if len(menuIDs) == 0 {
		return menuQueryNoAuth, []any{1, 0}
	}
	return menuQueryWithAuth, []any{1, 0, 1, menuIDs}
}

// adminUserEditParams 管理员用户编辑参数，字段使用指针支持部分更新。
type adminUserEditParams struct {
	Id          uint    // 用户 ID，0 表示新增
	Username    *string // 用户名
	Nickname    *string // 昵称
	Password    *string // 密码
	PhoneNumber *string // 手机号
	CountryCode *string // 国家代码
	Email       *string // 邮箱
	Status      *uint8  // 状态
	Avatar      *string // 头像
	DeptIds     *[]uint // 关联的部门 ID 列表
}
