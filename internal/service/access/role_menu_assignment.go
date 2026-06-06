package access

import (
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

// RoleMenuAssignmentService 负责规范角色菜单存储与历史清理。
type RoleMenuAssignmentService struct{}

// NewRoleMenuAssignmentService 创建角色菜单规范化服务实例。
func NewRoleMenuAssignmentService() *RoleMenuAssignmentService {
	return &RoleMenuAssignmentService{}
}

type roleMenuRuleSet struct {
	realMenuIDs map[uint]struct{}
}

// NormalizeMenuIDs 过滤出应写入 role_menu_map 的真实授权菜单。
func (s *RoleMenuAssignmentService) NormalizeMenuIDs(menuIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	rules, err := s.loadRuleSet(tx...)
	if err != nil {
		return nil, err
	}
	return rules.normalize(menuIDs), nil
}

// AllAssignableMenuIDs 返回当前系统全部应保存的真实授权菜单 ID。
func (s *RoleMenuAssignmentService) AllAssignableMenuIDs(tx ...*gorm.DB) ([]uint, error) {
	rules, err := s.loadRuleSet(tx...)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rules.realMenuIDs))
	for id := range rules.realMenuIDs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (s *RoleMenuAssignmentService) loadRuleSet(tx ...*gorm.DB) (*roleMenuRuleSet, error) {
	menuModel := model.NewMenu()
	if t := FirstTx(tx); t != nil {
		menuModel.SetDB(t)
	}
	infos, err := menuModel.AllRolePermissionInfos()
	if err != nil {
		return nil, err
	}

	assignableDescendants := make(map[uint]struct{})
	for _, info := range infos {
		if info.Status != global.Yes || info.IsAuth != global.Yes || info.Type == model.CATALOGUE {
			continue
		}
		for _, ancestorID := range parseAncestorIDs(info.Pids) {
			assignableDescendants[ancestorID] = struct{}{}
		}
	}

	realMenuIDs := make(map[uint]struct{})
	for _, info := range infos {
		if !isRealRoleMenu(info, assignableDescendants) {
			continue
		}
		realMenuIDs[info.ID] = struct{}{}
	}
	return &roleMenuRuleSet{realMenuIDs: realMenuIDs}, nil
}

func (r *roleMenuRuleSet) normalize(menuIDs []uint) []uint {
	if len(menuIDs) == 0 {
		return []uint{}
	}
	result := make([]uint, 0, len(menuIDs))
	seen := make(map[uint]struct{}, len(menuIDs))
	for _, id := range menuIDs {
		if _, ok := r.realMenuIDs[id]; !ok {
			continue
		}
		if _, duplicated := seen[id]; duplicated {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func isRealRoleMenu(info model.RolePermissionMenuInfo, assignableDescendants map[uint]struct{}) bool {
	if info.Status != global.Yes || info.IsAuth != global.Yes {
		return false
	}
	switch info.Type {
	case model.BUTTON:
		return true
	case model.MENU:
		_, hasDescendant := assignableDescendants[info.ID]
		return !hasDescendant
	default:
		return false
	}
}

func parseAncestorIDs(pids string) []uint {
	if pids == "" || pids == "0" {
		return nil
	}
	parts := strings.Split(pids, ",")
	result := make([]uint, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "0" {
			continue
		}
		var id uint
		for _, r := range part {
			if r < '0' || r > '9' {
				id = 0
				break
			}
			id = id*10 + uint(r-'0')
		}
		if id > 0 {
			result = append(result, id)
		}
	}
	return result
}
