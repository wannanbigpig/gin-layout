package resources

import (
	"github.com/samber/lo"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// DeptResources 表示部门树节点的响应结构。
type DeptResources struct {
	ID          uint             `json:"id"`
	Code        string           `json:"code"`
	IsSystem    uint8            `json:"is_system"`
	Pid         uint             `json:"pid"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Level       uint8            `json:"level"`
	Sort        uint16           `json:"sort"`
	ChildrenNum uint             `json:"children_num"`
	Children    []*DeptResources `json:"children,omitempty"`
	RoleList    []uint           `json:"role_list"`
	UserNumber  uint             `json:"user_number"`
	CreatedAt   utils.FormatDate `json:"created_at"`
	UpdatedAt   utils.FormatDate `json:"updated_at"`
}

// SetChildren 设置部门节点的子节点。
func (r *DeptResources) SetChildren(children []*DeptResources) {
	r.Children = children
}

// GetID 返回当前部门节点 ID。
func (r *DeptResources) GetID() uint {
	return r.ID
}

// GetPID 返回当前部门节点父级 ID。
func (r *DeptResources) GetPID() uint {
	return r.Pid
}

// DeptTreeTransformer 负责把部门模型转换为树形响应结构。
type DeptTreeTransformer struct {
	TreeResource[*model.Department, *DeptResources]
}

// SetCustomFields 填充部门资源的扩展字段。
func (r *DeptResources) SetCustomFields(data *model.Department) {
	r.RoleList = []uint{}
	if data == nil {
		return
	}
	r.Code = data.Code
	r.IsSystem = data.IsSystem
	if len(data.RoleList) > 0 {
		r.RoleList = lo.Map(data.RoleList, func(m model.DeptRoleMap, _ int) uint {
			return m.RoleId
		})
	}
}

// NewDeptTreeTransformer 创建部门树资源转换器。
func NewDeptTreeTransformer() DeptTreeTransformer {
	return DeptTreeTransformer{
		TreeResource: TreeResource[*model.Department, *DeptResources]{
			NewResource: func() *DeptResources {
				return &DeptResources{}
			},
		},
	}
}
