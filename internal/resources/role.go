package resources

import (
	"github.com/samber/lo"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// RoleResources 表示角色详情和树节点的响应结构。
type RoleResources struct {
	ID          uint             `json:"id"`
	Code        string           `json:"code"`
	IsSystem    uint8            `json:"is_system"`
	Pid         uint             `json:"pid"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Level       uint8            `json:"level"`
	Sort        uint16           `json:"sort"`
	ChildrenNum uint             `json:"children_num"`
	Status      uint8            `json:"status"`
	StatusName  string           `json:"status_name"` // 状态名称
	MenuList    []uint           `json:"menu_list"`
	CreatedAt   utils.FormatDate `json:"created_at"`
	UpdatedAt   utils.FormatDate `json:"updated_at"`
}

// GetID 返回当前角色节点 ID。
func (r *RoleResources) GetID() uint {
	return r.ID
}

// GetPID 返回当前角色节点父级 ID。
func (r *RoleResources) GetPID() uint {
	return r.Pid
}

// SetCustomFields 填充角色资源的扩展字段。
func (r *RoleResources) SetCustomFields(data *model.Role) {
	r.MenuList = []uint{}
	if data == nil {
		return
	}
	r.Code = data.Code
	r.IsSystem = data.IsSystem
	// 设置映射字段
	r.StatusName = data.StatusMap()
	r.MenuList = lo.Map(data.MenuList, func(m model.RoleMenuMap, _ int) uint {
		return m.MenuId
	})
}

// RoleTransformer 负责角色资源转换。
type RoleTransformer struct {
	BaseResources[*model.Role, *RoleResources]
}

// NewRoleTransformer 创建角色资源转换器。
func NewRoleTransformer() RoleTransformer {
	return RoleTransformer{
		BaseResources: BaseResources[*model.Role, *RoleResources]{
			NewResource: func() *RoleResources {
				return &RoleResources{}
			},
		},
	}
}
