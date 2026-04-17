package access

import "testing"

func BenchmarkBatchPermissionSync(b *testing.B) {
	roleIDs := []uint{11, 12, 13, 14}
	roleMenuMap := roleMenuIDMap{
		11: {101, 102, 103},
		12: {102, 104, 105},
		13: {106, 107, 108},
		14: {101, 108, 109},
	}
	enabledMenus := map[uint]struct{}{
		101: {}, 102: {}, 103: {}, 104: {}, 105: {}, 106: {}, 107: {}, 108: {}, 109: {},
	}
	menuPolicies := map[uint][][]string{
		101: {{"/admin/v1/user/list", "GET"}},
		102: {{"/admin/v1/user/create", "POST"}},
		103: {{"/admin/v1/user/update", "PUT"}},
		104: {{"/admin/v1/role/list", "GET"}},
		105: {{"/admin/v1/role/bind", "POST"}},
		106: {{"/admin/v1/dept/list", "GET"}},
		107: {{"/admin/v1/dept/update", "PUT"}},
		108: {{"/admin/v1/menu/list", "GET"}},
		109: {{"/admin/v1/menu/update", "PUT"}},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		policies := buildUserPolicies(roleIDs, roleMenuMap, enabledMenus, menuPolicies)
		if len(policies) == 0 {
			b.Fatal("expected policies")
		}
	}
}
