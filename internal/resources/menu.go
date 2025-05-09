package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

type MenuResources struct {
	ID                  uint             `json:"id"`
	Icon                string           `json:"icon"`              // 图标
	Title               string           `json:"title"`             // 中文标题
	Name                string           `json:"name"`              // 路由名称
	Code                string           `json:"code"`              // 前端权限标识
	Path                string           `json:"path"`              // 前端路由地址
	IsExternalLinks     int8             `json:"is_external_links"` // 是否外链 0:否 1:是
	IsExternalLinksName string           `json:"is_external_links_name,omitempty"`
	IsAuth              int8             `json:"is_auth"` // 是否鉴权 0:否 1:是
	Status              int8             `json:"status"`  // 状态，0正常 1禁用
	IsAuthName          string           `json:"is_auth_name,omitempty"`
	IsShow              int8             `json:"is_show"` // 是否显示，1是 0否
	IsShowName          string           `json:"is_show_name,omitempty"`
	IsNewWindow         int8             `json:"is_new_window"` // 是否新窗口打开, 1是 0否
	Level               int8             `json:"level"`         // 层级
	ISNewWindowName     string           `json:"is_new_window_name,omitempty"`
	Sort                int32            `json:"sort"` // 排序，数字越大，排名越靠前
	Type                int8             `json:"type"` // 菜单类型，1目录，2菜单，3按钮
	TypeName            string           `json:"type_name,omitempty"`
	Pid                 uint             `json:"pid"`              // 上级菜单id
	Desc                string           `json:"desc"`             // 描述
	AnimateEnter        string           `json:"animate_enter"`    // 进入动画
	AnimateLeave        string           `json:"animate_leave"`    // 离开动画
	AnimateDuration     float32          `json:"animate_duration"` // 动画时长
	CreatedAt           utils.FormatDate `json:"created_at"`
	UpdatedAt           utils.FormatDate `json:"updated_at"`
	Children            []*MenuResources `json:"children,omitempty"`
	ApiList             []uint           `json:"api_list"`
	Component           string           `json:"component"` // 前端组件路径
	Redirect            string           `json:"redirect"`  // 重定向地址
	FullPath            string           `json:"full_path"`
}

type MenuTransformer struct {
	BaseResources[*model.Menu, *MenuResources]
}

// NewMenuTransformer 实例化菜单资源转换器
func NewMenuTransformer() MenuTransformer {
	return MenuTransformer{
		BaseResources: BaseResources[*model.Menu, *MenuResources]{
			NewResource: func() *MenuResources {
				return &MenuResources{}
			},
		},
	}
}

// ToStruct 转换为单个资源
func (m MenuTransformer) ToStruct(data *model.Menu) *MenuResources {
	return buildMenuResource(data)
}

// ToCollection 转换为集合资源
func (m MenuTransformer) ToCollection(page, perPage int, total int64, data []*model.Menu) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		response = append(response, buildListMenuResource(v))
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}

// buildMenuResource 构造函数：将重复构建 MenuResources 的代码提取出来
func buildMenuResource(v *model.Menu) *MenuResources {
	return &MenuResources{
		ID:                  v.ID,
		Icon:                v.Icon,
		Title:               v.Title,
		Name:                v.Name,
		Component:           v.Component,
		Code:                v.Code,
		Path:                v.Path,
		FullPath:            v.FullPath,
		Redirect:            v.Redirect,
		IsExternalLinks:     v.IsExternalLinks,
		IsExternalLinksName: v.IsExternalLinksMap(),
		IsAuth:              v.IsAuth,
		Status:              v.Status,
		IsAuthName:          v.IsAuthMap(),
		IsShow:              v.IsShow,
		IsShowName:          v.IsShowMap(),
		IsNewWindow:         v.IsNewWindow,
		Level:               v.Level,
		ISNewWindowName:     v.IsNewWindowMap(),
		Sort:                v.Sort,
		Type:                v.Type,
		TypeName:            v.MenuTypeMap(),
		Pid:                 v.Pid,
		Desc:                v.Desc,
		AnimateEnter:        v.AnimateEnter,
		AnimateLeave:        v.AnimateLeave,
		AnimateDuration:     v.AnimateDuration,
		ApiList:             v.GetApiIds(),
		CreatedAt:           v.CreatedAt,
		UpdatedAt:           v.UpdatedAt,
	}
}

// MenuCollectionResources 菜单集合资源
type MenuCollectionResources struct {
	ID              uint                       `json:"id"`
	Icon            string                     `json:"icon"`
	Title           string                     `json:"title"`
	Code            string                     `json:"code"`
	Path            string                     `json:"path"`
	Redirect        string                     `json:"redirect"`
	Name            string                     `json:"name"`
	FullPath        string                     `json:"full_path"`
	Status          int8                       `json:"status"`
	IsAuth          int8                       `json:"is_auth"`
	IsShow          int8                       `json:"is_show"`
	IsNewWindow     int8                       `json:"is_new_window"`
	IsExternalLinks int8                       `json:"is_external_links"`
	Component       string                     `json:"component"`
	Sort            int32                      `json:"sort"`
	Type            int8                       `json:"type"`
	TypeName        string                     `json:"type_name,omitempty"`
	Pid             uint                       `json:"pid"`
	Desc            string                     `json:"desc"`
	Children        []*MenuCollectionResources `json:"children,omitempty"`
	CreatedAt       utils.FormatDate           `json:"created_at"`
	UpdatedAt       utils.FormatDate           `json:"updated_at"`
}

func (r *MenuCollectionResources) SetChildren(children []*MenuCollectionResources) {
	r.Children = children
}
func (r *MenuCollectionResources) GetID() uint {
	return r.ID
}
func (r *MenuCollectionResources) GetPID() uint {
	return r.Pid
}

type MenuTreeTransformer struct {
	TreeResource[*model.Menu, *MenuCollectionResources]
}

func NewMenuTreeTransformer() MenuTreeTransformer {
	return MenuTreeTransformer{
		TreeResource: TreeResource[*model.Menu, *MenuCollectionResources]{
			NewResource: func() *MenuCollectionResources {
				return &MenuCollectionResources{}
			},
		},
	}
}

func (r *MenuCollectionResources) SetCustomFields(data *model.Menu) {
	r.TypeName = data.MenuTypeMap()
}

// buildListMenuResource 构造函数：将重复构建 MenuCollectionResources 的代码提取出来
func buildListMenuResource(v *model.Menu) *MenuCollectionResources {
	return &MenuCollectionResources{
		ID:              v.ID,
		Icon:            v.Icon,
		Title:           v.Title,
		Code:            v.Code,
		Path:            v.Path,
		Redirect:        v.Redirect,
		Name:            v.Name,
		FullPath:        v.FullPath,
		Status:          v.Status,
		IsAuth:          v.IsAuth,
		IsShow:          v.IsShow,
		IsNewWindow:     v.IsNewWindow,
		IsExternalLinks: v.IsExternalLinks,
		Component:       v.Component,
		Sort:            v.Sort,
		Type:            v.Type,
		TypeName:        v.MenuTypeMap(),
		Pid:             v.Pid,
		Desc:            v.Desc,
		Children:        []*MenuCollectionResources{},
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
}
