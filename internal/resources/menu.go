package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// MenuBaseResources 菜单基础资源（公共字段）
type MenuBaseResources struct {
	ID              uint             `json:"id"`
	Icon            string           `json:"icon"`                  // 图标
	Title           string           `json:"title"`                 // 中文标题
	Name            string           `json:"name"`                  // 路由名称
	Code            string           `json:"code"`                  // 前端权限标识
	Path            string           `json:"path"`                  // 前端路由地址
	IsExternalLinks uint8            `json:"is_external_links"`     // 是否外链 0:否 1:是
	IsAuth          uint8            `json:"is_auth"`               // 是否鉴权 0:否 1:是
	Status          uint8            `json:"status"`                // 状态，0正常 1禁用
	StatusName      string           `json:"status_name,omitempty"` // 状态名称
	IsShow          uint8            `json:"is_show"`               // 是否显示，1是 0否
	IsNewWindow     uint8            `json:"is_new_window"`         // 是否新窗口打开, 1是 0否
	Sort            uint             `json:"sort"`                  // 排序，数字越大，排名越靠前
	Type            uint8            `json:"type"`                  // 菜单类型，1目录，2菜单，3按钮
	TypeName        string           `json:"type_name,omitempty"`
	Pid             uint             `json:"pid"`          // 上级菜单id
	ChildrenNum     uint             `json:"children_num"` // 子集数量
	Description     string           `json:"description"`  // 描述
	Component       string           `json:"component"`    // 前端组件路径
	Redirect        string           `json:"redirect"`     // 重定向地址
	FullPath        string           `json:"full_path"`
	CreatedAt       utils.FormatDate `json:"created_at"`
	UpdatedAt       utils.FormatDate `json:"updated_at"`
}

// MenuResources 菜单详情资源
type MenuResources struct {
	MenuBaseResources
	IsExternalLinksName string           `json:"is_external_links_name,omitempty"`
	IsAuthName          string           `json:"is_auth_name,omitempty"`
	IsShowName          string           `json:"is_show_name,omitempty"`
	ISNewWindowName     string           `json:"is_new_window_name,omitempty"`
	Level               uint8            `json:"level"`            // 层级
	AnimateEnter        string           `json:"animate_enter"`    // 进入动画
	AnimateLeave        string           `json:"animate_leave"`    // 离开动画
	AnimateDuration     float32          `json:"animate_duration"` // 动画时长
	Children            []*MenuResources `json:"children,omitempty"`
	ApiList             []uint           `json:"api_list"`
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

// buildMenuBaseResources 构建菜单基础资源（公共字段）
func buildMenuBaseResources(v *model.Menu) MenuBaseResources {
	return MenuBaseResources{
		ID:              v.ID,
		Icon:            v.Icon,
		Title:           v.Title,
		Name:            v.Name,
		Component:       v.Component,
		Code:            v.Code,
		Path:            v.Path,
		FullPath:        v.FullPath,
		Redirect:        v.Redirect,
		IsExternalLinks: v.IsExternalLinks,
		IsAuth:          v.IsAuth,
		Status:          v.Status,
		StatusName:      v.StatusMap(),
		IsShow:          v.IsShow,
		IsNewWindow:     v.IsNewWindow,
		Sort:            v.Sort,
		Type:            v.Type,
		TypeName:        v.MenuTypeMap(),
		Pid:             v.Pid,
		Description:     v.Description,
		ChildrenNum:     v.ChildrenNum,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
}

// buildMenuResource 构造函数：将重复构建 MenuResources 的代码提取出来
func buildMenuResource(v *model.Menu) *MenuResources {
	base := buildMenuBaseResources(v)
	return &MenuResources{
		MenuBaseResources:   base,
		IsExternalLinksName: v.IsExternalLinksMap(),
		IsAuthName:          v.IsAuthMap(),
		IsShowName:          v.IsShowMap(),
		ISNewWindowName:     v.IsNewWindowMap(),
		Level:               v.Level,
		AnimateEnter:        v.AnimateEnter,
		AnimateLeave:        v.AnimateLeave,
		AnimateDuration:     v.AnimateDuration,
		ApiList:             v.GetApiIds(),
	}
}

// MenuCollectionResources 菜单集合资源
type MenuCollectionResources struct {
	MenuBaseResources
	Children []*MenuCollectionResources `json:"children,omitempty"`
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
	base := buildMenuBaseResources(v)
	return &MenuCollectionResources{
		MenuBaseResources: base,
		Children:          []*MenuCollectionResources{},
	}
}
