package resources

import (
	"github.com/samber/lo"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// AdminUserResources 是后台管理员用户的响应资源结构体
// 用于对外暴露字段，避免直接返回数据库模型结构体
// 可配合脱敏规则处理敏感信息
type AdminUserResources struct {
	ID               uint             `json:"id"`                  // 管理员ID
	Nickname         string           `json:"nickname"`            // 昵称
	Username         string           `json:"username"`            // 用户名
	IsSuperAdmin     uint8            `json:"is_super_admin"`      // 是否为超级管理员
	IsSuperAdminName string           `json:"is_super_admin_name"` // 是否为超级管理员名称
	PhoneNumber      string           `json:"phone_number"`        // 手机号（可脱敏）
	CountryCode      string           `json:"country_code"`        // 国家区号
	Email            string           `json:"email"`               // 邮箱（可脱敏）
	Avatar           string           `json:"avatar"`              // 头像链接
	CreatedAt        utils.FormatDate `json:"created_at"`          // 创建时间
	UpdatedAt        utils.FormatDate `json:"updated_at"`          // 更新时间
	Status           int8             `json:"status"`              // 状态（1启用/2禁用）
	StatusName       string           `json:"status_name"`         // 状态名称
	LastIp           string           `json:"last_ip"`             // 上次登录 IP
	LastLogin        utils.FormatDate `json:"last_login"`          // 上次登录时间
	Departments      []department     `json:"departments"`         // 部门信息D
	RoleList         []uint           `json:"role_list"`           // 角色信息
}

// AdminUserTransformer 是 AdminUser 的资源转换器，实现 Resources 接口
// 内部嵌入 BaseResources 实现结构复用
type AdminUserTransformer struct {
	BaseResources[*model.AdminUser, *AdminUserResources]
}

func (r *AdminUserResources) SetCustomFields(data *model.AdminUser) {
	// 初始化 RoleList 和 Departments 为空切片，确保字段总是存在
	r.RoleList = []uint{}
	r.Departments = []department{}
	if data == nil {
		return
	}
	// 设置映射字段
	r.IsSuperAdminName = data.IsSuperAdminMap()
	r.StatusName = data.StatusMap()
	// 头像URL原样返回
	r.Avatar = data.Avatar
	// 如果 RoleList 有数据，则提取 RoleId
	if len(data.RoleList) > 0 {
		r.RoleList = lo.Map(data.RoleList, func(m model.AdminUserRoleMap, _ int) uint {
			return m.RoleId
		})
	}
	// 如果 Department 有数据，则转换为 department 结构
	if len(data.Department) > 0 {
		r.Departments = lo.Map(data.Department, func(d model.Department, _ int) department {
			return department{
				ID:   d.ID,
				Name: d.Name,
				Pid:  d.Pid,
			}
		})
	}
}

// NewAdminUserTransformer 返回 AdminUserTransformer 实例，绑定资源创建函数
func NewAdminUserTransformer() AdminUserTransformer {
	return AdminUserTransformer{
		BaseResources: BaseResources[*model.AdminUser, *AdminUserResources]{
			NewResource: func() *AdminUserResources {
				return &AdminUserResources{}
			},
		},
	}
}

// ToCollection 覆盖默认实现，支持手机号、邮箱等字段的自定义脱敏逻辑
// 若无特殊处理需求，可不实现该方法，默认继承 BaseResources 的逻辑
func (AdminUserTransformer) ToCollection(page, perPage int, total int64, data []*model.AdminUser) *Collection {
	response := make([]any, 0, len(data))
	phoneRule := utils.NewPhoneRule() // 手机号脱敏规则
	emailRule := utils.NewEmailRule() // 邮箱脱敏规则

	for _, v := range data {
		deptSlice := make([]department, 0, len(data))
		for _, d := range v.Department {
			deptSlice = append(deptSlice, department{
				ID:   d.ID,
				Name: d.Name,
				Pid:  d.Pid,
			})
		}

		response = append(response, &AdminUserResources{
			ID:               v.ID,
			Nickname:         v.Nickname,
			Username:         v.Username,
			IsSuperAdmin:     v.IsSuperAdmin,
			IsSuperAdminName: v.IsSuperAdminMap(),
			PhoneNumber:      phoneRule.Apply(v.PhoneNumber),
			CountryCode:      v.CountryCode,
			Email:            emailRule.Apply(v.Email),
			Avatar:           v.Avatar,
			Status:           v.Status,
			StatusName:       v.StatusMap(),
			LastIp:           v.LastIp,
			LastLogin:        v.LastLogin,
			CreatedAt:        v.CreatedAt,
			UpdatedAt:        v.UpdatedAt,
			Departments:      deptSlice,
		})
	}

	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}
