package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// MenuBaseResources 表示菜单响应的公共字段。
type MenuBaseResources struct {
	ID              uint             `json:"id"`
	Icon            string           `json:"icon"`                  // 图标
	Title           string           `json:"title,omitempty"`       // 当前请求语言标题
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

// MenuResources 表示菜单详情响应。
type MenuResources struct {
	MenuBaseResources
	TitleI18n           map[string]string `json:"title_i18n,omitempty"` // 多语言标题（仅详情返回）
	IsExternalLinksName string            `json:"is_external_links_name,omitempty"`
	IsAuthName          string            `json:"is_auth_name,omitempty"`
	IsShowName          string            `json:"is_show_name,omitempty"`
	ISNewWindowName     string            `json:"is_new_window_name,omitempty"`
	Level               uint8             `json:"level"`            // 层级
	AnimateEnter        string            `json:"animate_enter"`    // 进入动画
	AnimateLeave        string            `json:"animate_leave"`    // 离开动画
	AnimateDuration     float32           `json:"animate_duration"` // 动画时长
	Children            []*MenuResources  `json:"children,omitempty"`
	ApiList             []uint            `json:"api_list"`
}

// MenuTransformer 负责菜单详情资源转换。
type MenuTransformer struct {
	BaseResources[*model.Menu, *MenuResources]
}

// NewMenuTransformer 创建菜单资源转换器。
func NewMenuTransformer() MenuTransformer {
	return MenuTransformer{
		BaseResources: BaseResources[*model.Menu, *MenuResources]{
			NewResource: func() *MenuResources {
				return &MenuResources{}
			},
		},
	}
}

// ToStruct 将菜单模型转换为详情响应。
func (m MenuTransformer) ToStruct(data *model.Menu) *MenuResources {
	return m.ToStructWithTitles(data, "", nil)
}

// ToStructWithTitles 将菜单模型转换为详情响应，并显式注入标题信息。
func (m MenuTransformer) ToStructWithTitles(data *model.Menu, title string, titleI18n map[string]string) *MenuResources {
	return buildMenuResource(data, title, titleI18n)
}

// ToCollection 将菜单模型集合转换为分页响应。
func (m MenuTransformer) ToCollection(page, perPage int, total int64, data []*model.Menu) *Collection {
	return m.ToCollectionWithTitles(page, perPage, total, data, nil)
}

// ToCollectionWithTitles 将菜单模型集合转换为分页响应，并显式注入标题信息。
func (m MenuTransformer) ToCollectionWithTitles(page, perPage int, total int64, data []*model.Menu, titleByMenuID map[uint]string) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		response = append(response, buildListMenuResource(v, resolveTitleByMenuID(v.ID, titleByMenuID)))
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}

// buildMenuBaseResources 提取菜单响应的公共字段。
func buildMenuBaseResources(v *model.Menu, title string) MenuBaseResources {
	return MenuBaseResources{
		ID:              v.ID,
		Icon:            v.Icon,
		Title:           title,
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

// buildMenuResource 构建菜单详情响应。
func buildMenuResource(v *model.Menu, title string, titleI18n map[string]string) *MenuResources {
	base := buildMenuBaseResources(v, title)
	return &MenuResources{
		MenuBaseResources:   base,
		TitleI18n:           titleI18n,
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

// MenuCollectionResources 表示菜单树节点响应。
type MenuCollectionResources struct {
	MenuBaseResources
	Children []*MenuCollectionResources `json:"children,omitempty"`
}

// SetChildren 设置菜单树节点的子节点。
func (r *MenuCollectionResources) SetChildren(children []*MenuCollectionResources) {
	r.Children = children
}

// GetID 返回当前菜单节点 ID。
func (r *MenuCollectionResources) GetID() uint {
	return r.ID
}

// GetPID 返回当前菜单节点父级 ID。
func (r *MenuCollectionResources) GetPID() uint {
	return r.Pid
}

// SetCustomFields 填充菜单树节点的扩展字段。
func (r *MenuCollectionResources) SetCustomFields(data *model.Menu) {
	r.TypeName = data.MenuTypeMap()
}

// buildListMenuResource 构建菜单树节点响应。
func buildListMenuResource(v *model.Menu, title string) *MenuCollectionResources {
	base := buildMenuBaseResources(v, title)
	return &MenuCollectionResources{
		MenuBaseResources: base,
		Children:          []*MenuCollectionResources{},
	}
}

// BuildMenuTree 构建菜单树，并显式注入本地化标题。
func BuildMenuTree(data []*model.Menu, rootID uint, titleByMenuID map[uint]string) []*MenuCollectionResources {
	if len(data) == 0 {
		return []*MenuCollectionResources{}
	}

	parentMap := make(map[uint][]*model.Menu)
	for _, item := range data {
		if item == nil {
			continue
		}
		parentMap[item.Pid] = append(parentMap[item.Pid], item)
	}

	var build func(parentID uint) []*MenuCollectionResources
	build = func(parentID uint) []*MenuCollectionResources {
		children := parentMap[parentID]
		tree := make([]*MenuCollectionResources, 0, len(children))
		for _, menu := range children {
			node := buildListMenuResource(menu, resolveTitleByMenuID(menu.ID, titleByMenuID))
			node.TypeName = menu.MenuTypeMap()
			node.Children = build(menu.ID)
			tree = append(tree, node)
		}
		return tree
	}

	return build(rootID)
}

func resolveTitleByMenuID(menuID uint, titleByMenuID map[uint]string) string {
	if len(titleByMenuID) == 0 {
		return ""
	}
	return titleByMenuID[menuID]
}
