package resources

import (
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
	Status      uint8            `json:"status"`
	Children    []*RoleResources `json:"children"`
	CreatedAt   utils.FormatDate `json:"created_at"`
	UpdatedAt   utils.FormatDate `json:"updated_at"`
}

func (r *RoleResources) SetChildren(children []*RoleResources) {
	r.Children = children
}
func (r *RoleResources) GetID() uint {
	return r.ID
}
func (r *RoleResources) GetPID() uint {
	return r.Pid
}

type RoleTreeTransformer struct {
	TreeResource[*model.Role, *RoleResources]
}

func NewRoleTreeTransformer() RoleTreeTransformer {
	return RoleTreeTransformer{
		TreeResource: TreeResource[*model.Role, *RoleResources]{
			NewResource: func() *RoleResources {
				return &RoleResources{}
			},
		},
	}
}
