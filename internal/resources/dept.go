package resources

import (
	"github.com/samber/lo"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

type DeptResources struct {
	ID          uint             `json:"id"`
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

func (r *DeptResources) SetChildren(children []*DeptResources) {
	r.Children = children
}
func (r *DeptResources) GetID() uint {
	return r.ID
}
func (r *DeptResources) GetPID() uint {
	return r.Pid
}

type DeptTreeTransformer struct {
	TreeResource[*model.Department, *DeptResources]
}

func (r *DeptResources) SetCustomFields(data *model.Department) {
	// 初始化 RoleList 为空切片，确保字段总是存在
	r.RoleList = []uint{}
	if data == nil {
		return
	}
	// 如果 RoleList 有数据，则提取 RoleId
	if len(data.RoleList) > 0 {
		r.RoleList = lo.Map(data.RoleList, func(m model.DeptRoleMap, _ int) uint {
			return m.RoleId
		})
	}
}

func NewDeptTreeTransformer() DeptTreeTransformer {
	return DeptTreeTransformer{
		TreeResource: TreeResource[*model.Department, *DeptResources]{
			NewResource: func() *DeptResources {
				return &DeptResources{}
			},
		},
	}
}
