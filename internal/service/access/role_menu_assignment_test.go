package access

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestParseAncestorIDs(t *testing.T) {
	got := parseAncestorIDs("0,2,7")
	if len(got) != 2 || got[0] != 2 || got[1] != 7 {
		t.Fatalf("unexpected ancestor ids: %#v", got)
	}
}

func TestIsRealRoleMenu(t *testing.T) {
	descendants := map[uint]struct{}{
		7: {},
	}

	if !isRealRoleMenu(model.RolePermissionMenuInfo{ID: 78, Type: model.BUTTON, IsAuth: global.Yes, Status: global.Yes}, descendants) {
		t.Fatal("expected auth button to be a real role menu")
	}

	if isRealRoleMenu(model.RolePermissionMenuInfo{ID: 2, Type: model.CATALOGUE, IsAuth: global.Yes, Status: global.Yes}, descendants) {
		t.Fatal("expected catalogue menu to be filtered out")
	}

	if isRealRoleMenu(model.RolePermissionMenuInfo{ID: 7, Type: model.MENU, IsAuth: global.Yes, Status: global.Yes}, descendants) {
		t.Fatal("expected page parent with assignable descendants to be filtered out")
	}

	if !isRealRoleMenu(model.RolePermissionMenuInfo{ID: 5, Type: model.MENU, IsAuth: global.Yes, Status: global.Yes}, descendants) {
		t.Fatal("expected auth leaf page to be kept")
	}

	if isRealRoleMenu(model.RolePermissionMenuInfo{ID: 1, Type: model.MENU, IsAuth: global.No, Status: global.Yes}, descendants) {
		t.Fatal("expected login-visible menu to be filtered out")
	}
}

func TestRoleMenuRuleSetNormalizeKeepsOnlyRealMenuIDs(t *testing.T) {
	rules := &roleMenuRuleSet{
		realMenuIDs: map[uint]struct{}{
			34: {},
			78: {},
		},
	}

	got := rules.normalize([]uint{1, 2, 34, 34, 78})
	if len(got) != 2 || got[0] != 34 || got[1] != 78 {
		t.Fatalf("unexpected normalized menu ids: %#v", got)
	}
}
