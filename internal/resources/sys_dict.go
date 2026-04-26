package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// SysDictTypeResources 系统字典类型响应结构。
type SysDictTypeResources struct {
	ID           uint              `json:"id"`
	TypeCode     string            `json:"type_code"`
	TypeName     string            `json:"type_name"`
	TypeNameI18n map[string]string `json:"type_name_i18n,omitempty"`
	IsSystem     uint8             `json:"is_system"`
	Status       uint8             `json:"status"`
	Sort         uint              `json:"sort"`
	Remark       string            `json:"remark"`
	CreatedAt    utils.FormatDate  `json:"created_at"`
	UpdatedAt    utils.FormatDate  `json:"updated_at"`
}

// SysDictItemResources 系统字典项响应结构。
type SysDictItemResources struct {
	ID        uint              `json:"id"`
	TypeCode  string            `json:"type_code"`
	Label     string            `json:"label"`
	LabelI18n map[string]string `json:"label_i18n,omitempty"`
	Value     string            `json:"value"`
	Color     string            `json:"color"`
	TagType   string            `json:"tag_type"`
	IsDefault uint8             `json:"is_default"`
	IsSystem  uint8             `json:"is_system"`
	Status    uint8             `json:"status"`
	Sort      uint              `json:"sort"`
	Remark    string            `json:"remark"`
	CreatedAt utils.FormatDate  `json:"created_at"`
	UpdatedAt utils.FormatDate  `json:"updated_at"`
}

// SysDictOptionResources 前端下拉选项响应结构。
type SysDictOptionResources struct {
	Label     string `json:"label"`
	Value     string `json:"value"`
	Color     string `json:"color"`
	TagType   string `json:"tag_type"`
	IsDefault uint8  `json:"is_default"`
}

type SysDictTypeTransformer struct {
	BaseResources[*model.SysDictType, *SysDictTypeResources]
}

type SysDictItemTransformer struct {
	BaseResources[*model.SysDictItem, *SysDictItemResources]
}

func NewSysDictTypeTransformer() SysDictTypeTransformer {
	return SysDictTypeTransformer{
		BaseResources: BaseResources[*model.SysDictType, *SysDictTypeResources]{
			NewResource: func() *SysDictTypeResources {
				return &SysDictTypeResources{}
			},
		},
	}
}

func NewSysDictItemTransformer() SysDictItemTransformer {
	return SysDictItemTransformer{
		BaseResources: BaseResources[*model.SysDictItem, *SysDictItemResources]{
			NewResource: func() *SysDictItemResources {
				return &SysDictItemResources{}
			},
		},
	}
}

func ToSysDictOptions(items []model.SysDictItem) []SysDictOptionResources {
	options := make([]SysDictOptionResources, 0, len(items))
	for _, item := range items {
		options = append(options, SysDictOptionResources{
			Label:     item.Label,
			Value:     item.Value,
			Color:     item.Color,
			TagType:   item.TagType,
			IsDefault: item.IsDefault,
		})
	}
	return options
}
