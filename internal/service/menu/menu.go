package menu

import "github.com/wannanbigpig/gin-layout/internal/service"

const (
	menuRootPid   = "0"
	menuRootLevel = 1
	maxMenuLevel  = 4 // 最多4层菜单
	allStatus     = 2
	rootPath      = "/"
)

// MenuService 菜单服务
type MenuService struct {
	service.Base
}

// NewMenuService 创建菜单服务实例
func NewMenuService() *MenuService {
	return &MenuService{}
}
