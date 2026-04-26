package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// SysConfigResources 系统参数响应结构。
type SysConfigResources struct {
	ID             uint              `json:"id"`
	ConfigKey      string            `json:"config_key"`
	ConfigName     string            `json:"config_name"`
	ConfigNameI18n map[string]string `json:"config_name_i18n,omitempty"`
	ConfigValue    string            `json:"config_value"`
	ValueType      string            `json:"value_type"`
	GroupCode      string            `json:"group_code"`
	IsSystem       uint8             `json:"is_system"`
	IsSensitive    uint8             `json:"is_sensitive"`
	Status         uint8             `json:"status"`
	Sort           uint              `json:"sort"`
	Remark         string            `json:"remark"`
	CreatedAt      utils.FormatDate  `json:"created_at"`
	UpdatedAt      utils.FormatDate  `json:"updated_at"`
}

// SysConfigTransformer 负责系统参数资源转换。
type SysConfigTransformer struct {
	BaseResources[*model.SysConfig, *SysConfigResources]
}

func NewSysConfigTransformer() SysConfigTransformer {
	return SysConfigTransformer{
		BaseResources: BaseResources[*model.SysConfig, *SysConfigResources]{
			NewResource: func() *SysConfigResources {
				return &SysConfigResources{}
			},
		},
	}
}
