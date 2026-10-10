package admin

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// AdminUserService 授权服务。
type AdminUserService struct {
	service.Base
}

const (
	menuQuerySuperAdmin = "status = ?"
	menuQueryNoAuth     = "status = ? AND is_auth = ?"
	menuQueryWithAuth   = "status = ? AND (is_auth = ? OR (is_auth = ? AND id IN (?)))"
	maxAvatarSize       = 2 * 1024 * 1024
)

// NewAdminUserService 创建管理员用户服务实例。
func NewAdminUserService() *AdminUserService {
	return &AdminUserService{}
}

func (s *AdminUserService) handleMutationError(err error, fallbackCode int) error {
	if err == nil {
		return nil
	}

	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) {
		return businessErr
	}

	log.Logger.Error("操作失败，原始错误已记录", zap.Error(err), zap.Int("fallback_code", fallbackCode))
	return e.NewBusinessError(fallbackCode)
}

var prepareUserTokenRevocationFunc = func(tx *gorm.DB, userID uint, revokedCode uint8, revokedReason string) (*auth.PreparedUserTokenRevocation, error) {
	return auth.NewLoginService().PrepareUserTokenRevocation(tx, userID, revokedCode, revokedReason)
}

var completeUserTokenRevocationFunc = func(prepared *auth.PreparedUserTokenRevocation) error {
	return auth.NewLoginService().CompleteUserTokenRevocation(prepared)
}

type adminDataScopeFilter interface {
	Build(ctx context.Context, input access.DataScopeInput, deptCol, ownerCol string) (string, []any)
	ResolveDepartmentScope(ctx context.Context, input access.DataScopeInput) (bool, []uint, error)
}

var newAdminDataScopeFilter = func() adminDataScopeFilter {
	return access.NewDataScopeFilter()
}

func (s *AdminUserService) prepareUserTokenRevocation(tx *gorm.DB, item *userTokenRevocation) error {
	prepared, err := prepareUserTokenRevocationFunc(tx, item.userID, item.revokedCode, item.revokedReason)
	if err != nil {
		log.Logger.Error("撤销用户token失败", zap.Error(err), zap.Uint("user_id", item.userID))
		return err
	}
	item.prepared = prepared
	return nil
}

type userTokenRevocation struct {
	userID        uint
	revokedCode   uint8
	revokedReason string
	prepared      *auth.PreparedUserTokenRevocation
}

func (s *AdminUserService) revokeUserTokensAfterCommit(items []userTokenRevocation) error {
	var errs []error
	for _, item := range items {
		if err := completeUserTokenRevocationFunc(item.prepared); err != nil {
			log.Logger.Error("同步用户token撤销状态失败", zap.Error(err), zap.Uint("user_id", item.userID))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// GetUserInfo 获取用户信息。
func (s *AdminUserService) GetUserInfo(id uint) (*resources.AdminUserResources, error) {
	adminUsersModel := model.NewAdminUsers()
	err := adminUsersModel.GetByIdWithPreload(id, "RoleList", "Department")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.UserDoesNotExist)
		}
		return nil, err
	}

	return resources.NewAdminUserTransformer().ToStruct(adminUsersModel), nil
}

// GetManagedUserInfo 在当前操作人的数据权限内获取管理员详情。
func (s *AdminUserService) GetManagedUserInfo(id uint) (*resources.AdminUserResources, error) {
	if err := s.requireUserInDataScope(id); err != nil {
		return nil, err
	}
	return s.GetUserInfo(id)
}

// GetUserMenuInfo 获取用户权限信息。
func (s *AdminUserService) GetUserMenuInfo(id uint, locale string) (any, error) {
	adminUsersModel := model.NewAdminUsers()
	menuModel := model.NewMenu()
	menuI18nModel := model.NewMenuI18n()
	return s.getUserMenuInfoWithModels(id, locale, adminUsersModel, menuModel, menuI18nModel)
}

func (s *AdminUserService) getUserMenuInfoWithModels(id uint, locale string, adminUsersModel *model.AdminUser, menuModel *model.Menu, menuI18nModel *model.MenuI18n) (any, error) {
	err := adminUsersModel.GetById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.UserDoesNotExist)
		}
		return nil, err
	}
	isSuperAdmin := id == global.SuperAdminId || adminUsersModel.IsSuperAdmin == global.Yes
	condition, args := s.userMenuQuery(isSuperAdmin, nil)
	if !isSuperAdmin {
		menuIDs, err := access.NewPermissionSyncCoordinator().AccessibleMenuIDs(id, true)
		if err != nil {
			return nil, err
		}
		condition, args = s.userMenuQuery(false, menuIDs)
	}

	menuDB, err := menuModel.GetDB(menuModel)
	if err != nil {
		return nil, err
	}
	if condition != "" {
		menuDB = menuDB.Where(condition, args...)
	}
	var menus []*model.Menu
	if err := menuDB.Order("sort desc, id desc").Find(&menus).Error; err != nil {
		return nil, err
	}

	menuIDs := make([]uint, 0, len(menus))
	for _, menu := range menus {
		if menu == nil {
			continue
		}
		menuIDs = append(menuIDs, menu.ID)
	}
	titleMap, err := menuI18nModel.LocalizedTitleMapByMenuIDs(menuIDs, []string{
		i18n.NormalizeLocale(locale),
		i18n.LocaleZhCN,
		i18n.LocaleEnUS,
	})
	if err != nil {
		return nil, err
	}
	return resources.BuildMenuTree(menus, 0, titleMap), nil
}

// List 返回管理员分页列表。
func (s *AdminUserService) List(params *form.AdminUserList) (*resources.Collection, error) {
	conditionStr, args := s.buildListCondition(params)

	// 接入数据权限过滤
	dsCond, dsArgs := s.buildDataScopeCondition()
	if dsCond != "" {
		if conditionStr != "" {
			conditionStr = conditionStr + " AND " + dsCond
		} else {
			conditionStr = dsCond
		}
		args = append(args, dsArgs...)
	}

	adminUserModel := model.NewAdminUsers()
	listOptions := s.adminUserListOptions()
	listOptions.OrderBy = query_builder.ResolveListOrder(params.SortBy, params.SortOrder, listOptions.OrderBy, "id", "username", "nickname", "created_at", "updated_at")
	total, collection, err := model.ListPageE(adminUserModel, params.Page, params.PerPage, conditionStr, args, listOptions)
	if err != nil {
		log.Logger.Error("查询管理员列表失败", zap.Error(err))
		return nil, err
	}
	return resources.NewAdminUserTransformer().ToCollection(params.Page, params.PerPage, total, collection), nil
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
	var userID any
	if params.ID > 0 {
		userID = params.ID
	}
	query := query_builder.New().
		AddLike("username", params.UserName).
		AddEq("id", userID).
		AddLike("nickname", params.NickName).
		AddLike("email", params.Email).
		AddLike("full_phone_number", params.PhoneNumber).
		AddEq("status", params.Status)
	if params.DeptId > 0 {
		query.AddCondition(
			"EXISTS (SELECT 1 FROM admin_user_department_map WHERE admin_user_department_map.uid = admin_user.id AND admin_user_department_map.dept_id = ?)",
			params.DeptId,
		)
	}
	return query.Build()
}

// buildDataScopeCondition 构建数据权限过滤条件。
// 管理员和部门是多对多关系（通过 admin_user_department_map 表），使用 EXISTS 子查询过滤。
func (s *AdminUserService) buildDataScopeCondition() (string, []any) {
	filter := newAdminDataScopeFilter()
	cond, filterArgs := filter.Build(s.context(), s.dataScopeInput(), "admin_user_department_map.dept_id", "admin_user.id")
	if cond == "" {
		return "", nil
	}

	return wrapAdminUserDeptScopeCondition(cond), filterArgs
}

func (s *AdminUserService) dataScopeInput() access.DataScopeInput {
	uid := s.GetAdminUserId()
	isSuperAdmin := uid == global.SuperAdminId
	if ctx := s.GetCtx(); ctx != nil {
		if principal := auth.GetAuthPrincipal(ctx); principal != nil {
			isSuperAdmin = principal.IsSuperAdmin == global.Yes
		}
	}
	return access.DataScopeInput{UID: uid, IsSuperAdmin: isSuperAdmin}
}

// requireDepartmentsInDataScope 确保写入的部门都属于当前操作人的合并数据范围。
func (s *AdminUserService) requireDepartmentsInDataScope(deptIDs []uint) error {
	deptIDs = access.UniqueUintSlice(deptIDs)
	if len(deptIDs) == 0 {
		return nil
	}

	scopeAll, allowedDeptIDs, err := newAdminDataScopeFilter().ResolveDepartmentScope(s.context(), s.dataScopeInput())
	if err != nil {
		return err
	}
	if scopeAll {
		return nil
	}

	allowed := make(map[uint]struct{}, len(allowedDeptIDs))
	for _, deptID := range allowedDeptIDs {
		allowed[deptID] = struct{}{}
	}
	for _, deptID := range deptIDs {
		if _, ok := allowed[deptID]; !ok {
			return e.NewBusinessError(e.AuthorizationErr)
		}
	}
	return nil
}

// requireUserInDataScope 确保目标用户属于当前操作人可管理的数据范围。
func (s *AdminUserService) requireUserInDataScope(id uint) error {
	condition, args := s.buildDataScopeCondition()
	if condition == "" {
		return nil
	}
	db, err := model.NewAdminUsers().GetDB()
	if err != nil {
		return err
	}
	allowed, err := adminUserInDataScope(db, id, condition, args)
	if err != nil {
		return err
	}
	if !allowed {
		return e.NewBusinessError(e.AuthorizationErr)
	}
	return nil
}

func adminUserInDataScope(db *gorm.DB, id uint, condition string, args []any) (bool, error) {
	var allowed bool
	query := db.Table(model.NewAdminUsers().TableName()).
		Select("1").
		Where("admin_user.id = ? AND admin_user.deleted_at = 0", id)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	if err := query.Limit(1).Scan(&allowed).Error; err != nil {
		return false, err
	}
	return allowed, nil
}

func wrapAdminUserDeptScopeCondition(cond string) string {
	deptPredicate := "admin_user_department_map.dept_id IN (?)"
	deptExists := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM admin_user_department_map WHERE admin_user_department_map.uid = admin_user.id AND %s)",
		deptPredicate,
	)
	return strings.ReplaceAll(cond, deptPredicate, deptExists)
}

// context 返回标准 context.Context，用于 DataScopeFilter 等需要 context.Context 的场景。
func (s *AdminUserService) context() context.Context {
	if ctx := s.GetCtx(); ctx != nil && ctx.Request != nil {
		return ctx.Request.Context()
	}
	return context.Background()
}

// adminUserOptionsLimit 选择器远程搜索返回的最大条数。
const adminUserOptionsLimit = 20

// Options 按账号/昵称模糊搜索管理员，供前端用户选择器远程搜索（仅返回最小字段，限量返回）。
func (s *AdminUserService) Options(keyword string) ([]resources.AdminUserOptionResources, error) {
	conditionStr, args := query_builder.New().
		AddKeywordLike(keyword, "username", "nickname").
		Build()
	_, collection, err := model.ListPageE(model.NewAdminUsers(), 1, adminUserOptionsLimit, conditionStr, args, model.ListOptionalParams{
		SelectFields: []string{"id", "username", "nickname"},
		OrderBy:      "id asc",
	})
	if err != nil {
		log.Logger.Error("查询管理员选项失败", zap.Error(err))
		return nil, err
	}
	return resources.ToAdminUserOptions(collection), nil
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
	Id                                   uint    // 用户 ID，0 表示新增
	Username                             *string // 用户名
	Nickname                             *string // 昵称
	Password                             *string // 密码
	PhoneNumber                          *string // 手机号
	CountryCode                          *string // 国家代码
	Email                                *string // 邮箱
	Status                               *uint8  // 状态
	IsSuperAdmin                         *uint8  // 是否超级管理员
	Avatar                               *string // 头像
	DeptIds                              *[]uint // 关联的部门 ID 列表
	allowDefaultSuperAdminPasswordChange bool    // 仅允许默认超级管理员通过个人资料入口修改自己的密码
}

// UploadAvatar 专门用于上传用户头像。
// 注意：
//  1. 此接口会将文件统一存放至隐藏目录 (system_avatars)。
//  2. 结合后端的 FileReferenceService 机制，当用户更换新头像时，系统仅会释放对旧头像的“引用计数”。
//  3. 目前系统出于防误删策略，并未配置针对“无引用闲置图片”的自动物理清理定时任务。
//     这意味着被替换掉的旧头将会作为历史闲置数据长期保留在数据库及存储介质中。
//     若业务对存储空间敏感，未来需自行补充如 `cron:clean-unused-files` 的定时扫描和物理清理任务。
func (s *AdminUserService) UploadAvatar(ctx context.Context, uid uint, fileHeader *multipart.FileHeader) (*utils.FileInfo, error) {
	if err := validateAvatarFile(fileHeader); err != nil {
		return nil, err
	}

	fileResourceService := service.NewFileResourceService()
	folderID, err := fileResourceService.EnsureAvatarFolder(ctx)
	if err != nil {
		log.Logger.Error("获取头像存储目录失败", zap.Error(err))
		return nil, e.NewBusinessError(e.ServerErr)
	}

	commonService := service.NewCommonService()
	commonService.SetAdminUserId(uid)

	// 为解决同一个系统隐藏目录下同名文件冲突 (uniq_folder_name_deleted 唯一索引)
	// 在上传前强行将文件名改为基于 uid 和时间戳的唯一名。
	ext := filepath.Ext(fileHeader.Filename)
	fileHeader.Filename = fmt.Sprintf("avatar_%d_%d%s", uid, time.Now().UnixNano(), ext)

	// 调用 CommonService 的扩展上传方法，强制指定 isPublic=1, uploadScene="avatar"
	fileInfo, err := commonService.UploadImageEx(fileHeader, true, "avatar", folderID, "avatar")
	if err != nil {
		log.Logger.Error("头像上传保存失败", zap.Error(err))
		return nil, err
	}
	return fileInfo, nil
}

func validateAvatarFile(fileHeader *multipart.FileHeader) error {
	if fileHeader == nil {
		return e.NewBusinessError(e.InvalidParameter, "头像文件不能为空")
	}
	if fileHeader.Size > maxAvatarSize {
		return e.NewBusinessError(e.InvalidParameter, "头像大小不能大于2MB")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return e.NewBusinessError(e.InvalidParameter, "头像文件读取失败")
	}
	defer file.Close()

	if _, allowed, err := utils.IsAllowedImageMIME(file, map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}); err != nil {
		return e.NewBusinessError(e.InvalidParameter, "头像文件类型检测失败")
	} else if !allowed {
		return e.NewBusinessError(e.InvalidParameter, "仅支持jpg/jpeg/png/webp头像")
	}
	return nil
}
