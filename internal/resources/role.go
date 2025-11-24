package resources

import (
	"github.com/samber/lo"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

type RoleResources struct {
	ID          uint             `json:"id"`
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

func (r *RoleResources) GetID() uint {
	return r.ID
}
func (r *RoleResources) GetPID() uint {
	return r.Pid
}

func (r *RoleResources) SetCustomFields(data *model.Role) {
	r.MenuList = []uint{}
	if data == nil {
		return
	}
	// 设置映射字段
	r.StatusName = data.StatusMap()
	r.MenuList = lo.Map(data.MenuList, func(m model.RoleMenuMap, _ int) uint {
		return m.MenuId
	})
}

type RoleTransformer struct {
	BaseResources[*model.Role, *RoleResources]
}

func NewRoleTransformer() RoleTransformer {
	return RoleTransformer{
		BaseResources: BaseResources[*model.Role, *RoleResources]{
			NewResource: func() *RoleResources {
				return &RoleResources{}
			},
		},
	}
}
