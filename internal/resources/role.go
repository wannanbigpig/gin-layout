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
	CreatedAt   utils.FormatDate `json:"created_at"`
	UpdatedAt   utils.FormatDate `json:"updated_at"`
}

// RoleTransformer 角色资源转换
type RoleTransformer struct {
	BaseResources[*model.Role, *RoleResources]
}

// NewRoleTransformer 实例化权限资源转换器
func NewRoleTransformer() RoleTransformer {
	return RoleTransformer{
		BaseResources: BaseResources[*model.Role, *RoleResources]{
			NewResource: func() *RoleResources {
				return &RoleResources{}
			},
		},
	}
}
