package model

import (
	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
)

// Menu 权限路由表
type Menu struct {
	ContainsDeleteBaseModel
	Icon            string       `json:"icon"`              // 图标
	Title           string       `json:"title"`             // 中文标题
	Code            string       `json:"code"`              // 前端权限标识
	Path            string       `json:"path"`              // 前端路由
	FullPath        string       `json:"full_path"`         // 完整前端路由
	IsShow          uint8        `json:"is_show"`           // 是否显示，1是 0否
	IsNewWindow     uint8        `json:"is_new_window"`     // 是否新窗口打开, 1是 0否
	Sort            uint         `json:"sort"`              // 排序，数字越大，排名越靠前
	Type            uint8        `json:"type"`              // 菜单类型，1目录，2菜单，3按钮
	Pid             uint         `json:"pid"`               // 上级菜单id
	Level           uint8        `json:"level"`             // 层级
	Pids            string       `json:"pids"`              // 层级序列，多个用英文逗号隔开
	ChildrenNum     uint         `json:"children_num"`      // 子集数量
	Description     string       `json:"description"`       // 描述
	IsAuth          uint8        `json:"is_auth"`           // 是否鉴权 0:否 1:是
	IsExternalLinks uint8        `json:"is_external_links"` // 是否外链 0:否 1:是
	Name            string       `json:"name"`              // 路由名称
	Component       string       `json:"component"`         // 组件路径
	AnimateEnter    string       `json:"animate_enter"`     // 进入动画
	AnimateLeave    string       `json:"animate_leave"`     // 离开动画
	AnimateDuration float32      `json:"animate_duration"`  // 动画时长
	ApiList         []MenuApiMap `json:"api_list" gorm:"foreignkey:menu_id;references:id"`
	Status          uint8        `json:"status"`   // 状态，0禁用，1启用
	Redirect        string       `json:"redirect"` // 重定向路由名称
}

const CATALOGUE uint8 = 1
const MENU uint8 = 2
const BUTTON uint8 = 3

var MenuType modelDict.Dict = map[uint8]string{
	CATALOGUE: "目录",
	MENU:      "菜单",
	BUTTON:    "按钮",
}

func (m *Menu) MenuTypeMap() string {
	return MenuType.Map(m.Type)
}

func (m *Menu) IsExternalLinksMap() string {
	return modelDict.IsMap.Map(m.IsExternalLinks)
}

func (m *Menu) IsAuthMap() string {
	return modelDict.IsMap.Map(m.IsAuth)
}

func (m *Menu) IsShowMap() string {
	return modelDict.IsMap.Map(m.IsShow)
}

func (m *Menu) IsNewWindowMap() string {
	return modelDict.IsMap.Map(m.IsNewWindow)
}

// StatusMap 状态映射
func (m *Menu) StatusMap() string {
	return modelDict.IsMap.Map(m.Status)
}

func (m *Menu) GetApiIds() []uint {
	// 如果 ApiList 为空，直接返回空切片
	if len(m.ApiList) == 0 {
		return []uint{}
	}

	// 预分配切片容量，避免多次内存分配
	apiIds := make([]uint, 0, len(m.ApiList))
	for _, v := range m.ApiList {
		apiIds = append(apiIds, v.ApiId)
	}
	return apiIds
}

func NewMenu() *Menu {
	return &Menu{}
}

// TableName 获取表名
func (m *Menu) TableName() string {
	return "a_menu"
}
