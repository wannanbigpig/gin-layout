package access

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserPermissionSyncForEachUserDeduplicatesInOrder(t *testing.T) {
	service := NewUserPermissionSyncService()
	var visited []uint

	err := service.forEachUser([]uint{2, 5, 2, 0, 5}, func(userID uint) error {
		visited = append(visited, userID)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []uint{2, 5, 0}
	if !reflect.DeepEqual(visited, want) {
		t.Fatalf("unexpected visit order: got %v want %v", visited, want)
	}
}

func TestUserPermissionSyncForEachUserStopsOnError(t *testing.T) {
	service := NewUserPermissionSyncService()
	wantErr := errors.New("stop")

	err := service.forEachUser([]uint{1, 2, 3}, func(userID uint) error {
		if userID == 2 {
			return wantErr
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestUserPermissionSyncSyncUsersSkipsEmptyUserSet(t *testing.T) {
	reloadCount := 0
	service := NewUserPermissionSyncServiceWithDeps(UserPermissionSyncServiceDeps{
		ReloadPolicy: func() error {
			reloadCount++
			return nil
		},
	})

	if err := service.SyncUsers(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reloadCount != 0 {
		t.Fatalf("expected reload to be skipped, got %d", reloadCount)
	}
}

func TestExpandUserRolesKeepsOnlyDirectAndDeptRoles(t *testing.T) {
	got, allRoleIDs := expandUserRoles(map[uint][]uint{
		10: {2, 3, 2},
		11: {4},
	})

	if !reflect.DeepEqual(got[10], []uint{2, 3}) {
		t.Fatalf("unexpected roles for user 10: %v", got[10])
	}
	if !reflect.DeepEqual(got[11], []uint{4}) {
		t.Fatalf("unexpected roles for user 11: %v", got[11])
	}
	roleSet := make(map[uint]struct{}, len(allRoleIDs))
	for _, roleID := range allRoleIDs {
		roleSet[roleID] = struct{}{}
	}
	if len(roleSet) != 3 {
		t.Fatalf("unexpected all role ids: %v", allRoleIDs)
	}
	for _, roleID := range []uint{2, 3, 4} {
		if _, ok := roleSet[roleID]; !ok {
			t.Fatalf("unexpected all role ids: %v", allRoleIDs)
		}
	}
	if len(allRoleIDs) != 3 {
		t.Fatalf("unexpected all role ids: %v", allRoleIDs)
	}
}

func TestFilterUserRolesByEnabledSetDropsDisabledRoles(t *testing.T) {
	got, allRoleIDs := filterUserRolesByEnabledSet(map[uint][]uint{
		10: {2, 3, 4},
		11: {4, 5},
	}, map[uint]struct{}{
		2: {},
		4: {},
	})

	if !reflect.DeepEqual(got[10], []uint{2, 4}) {
		t.Fatalf("unexpected roles for user 10: %v", got[10])
	}
	if !reflect.DeepEqual(got[11], []uint{4}) {
		t.Fatalf("unexpected roles for user 11: %v", got[11])
	}
	if !reflect.DeepEqual(allRoleIDs, []uint{2, 4}) {
		t.Fatalf("unexpected all role ids: %v", allRoleIDs)
	}
}

func TestPermissionSyncCoordinatorRunAfterCommitReloadsOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	reloadCount := 0
	coordinator := NewPermissionSyncCoordinatorWithDeps(PermissionSyncCoordinatorDeps{
		Syncer: NewUserPermissionSyncServiceWithDeps(UserPermissionSyncServiceDeps{
			ReloadPolicy: func() error {
				reloadCount++
				return nil
			},
		}),
	})

	err = coordinator.RunAfterCommit(db, "reload failed", func(tx *gorm.DB) error {
		if tx == nil {
			t.Fatal("expected transaction")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reloadCount != 1 {
		t.Fatalf("expected reload once, got %d", reloadCount)
	}
}

func TestPermissionSyncCoordinatorRunAfterCommitSwallowsReloadFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	coordinator := NewPermissionSyncCoordinatorWithDeps(PermissionSyncCoordinatorDeps{
		Syncer: NewUserPermissionSyncServiceWithDeps(UserPermissionSyncServiceDeps{
			ReloadPolicy: func() error {
				return errors.New("reload failed")
			},
		}),
	})

	err = coordinator.RunAfterCommit(db, "reload failed", func(tx *gorm.DB) error {
		if tx == nil {
			t.Fatal("expected transaction")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil even when reload fails after commit, got %v", err)
	}
}

func TestPermissionSyncCoordinatorReloadPolicyCacheWithCodeSwallowsReloadFailure(t *testing.T) {
	coordinator := NewPermissionSyncCoordinatorWithDeps(PermissionSyncCoordinatorDeps{
		Syncer: NewUserPermissionSyncServiceWithDeps(UserPermissionSyncServiceDeps{
			ReloadPolicy: func() error {
				return errors.New("reload failed")
			},
		}),
	})

	if err := coordinator.ReloadPolicyCacheWithCode(12345); err != nil {
		t.Fatalf("expected nil when reload fails after commit, got %v", err)
	}
}

func TestPermissionInitializationClosureBuildsUserPolicies(t *testing.T) {
	db := newPermissionClosureTestDB(t)
	execPermissionClosureSQL(t, db, "INSERT INTO admin_user (id, status, is_super_admin, deleted_at) VALUES (2, 1, 0, 0)")
	execPermissionClosureSQL(t, db, "INSERT INTO role (id, status, deleted_at) VALUES (10, 1, 0)")
	execPermissionClosureSQL(t, db, "INSERT INTO admin_user_role_map (uid, role_id) VALUES (2, 10)")
	execPermissionClosureSQL(t, db, `INSERT INTO menu (id, code, pids, type, is_auth, status, deleted_at) VALUES
		(100, 'demo:page', '0', 2, 1, 1, 0),
		(101, 'demo:read', '0,100', 3, 1, 1, 0),
		(102, 'demo:leaf', '0', 2, 1, 1, 0)`)
	execPermissionClosureSQL(t, db, `INSERT INTO api (id, route, method, is_auth, is_effective, deleted_at) VALUES
		(201, '/demo/read', 'GET', 2, 1, 0),
		(202, '/demo/leaf', 'POST', 2, 1, 0)`)

	defaults := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		Bindings: []defaultMenuAPIBinding{
			{MenuCode: "demo:read", Route: "/demo/read", Method: "GET"},
			{MenuCode: "demo:leaf", Route: "/demo/leaf", Method: "POST"},
		},
	})
	if err := defaults.Sync(db); err != nil {
		t.Fatalf("sync default menu api mappings: %v", err)
	}

	assignableMenuIDs, err := NewRoleMenuAssignmentService().AllAssignableMenuIDs(db)
	if err != nil {
		t.Fatalf("load assignable menu ids: %v", err)
	}
	if !reflect.DeepEqual(assignableMenuIDs, []uint{101, 102}) {
		t.Fatalf("expected only real permission nodes, got %v", assignableMenuIDs)
	}
	for _, menuID := range assignableMenuIDs {
		execPermissionClosureSQL(t, db, "INSERT INTO role_menu_map (role_id, menu_id) VALUES (10, ?)", menuID)
	}

	policiesByUser, err := NewUserPermissionSyncService().collectPoliciesForUsers([]uint{2}, db)
	if err != nil {
		t.Fatalf("collect user policies: %v", err)
	}
	assertPolicySet(t, policiesByUser[2], []string{
		"GET /demo/read",
		"POST /demo/leaf",
	})
}

func newPermissionClosureTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	statements := []string{
		`CREATE TABLE admin_user (id integer primary key, status integer not null default 1, is_super_admin integer not null default 0, deleted_at integer not null default 0)`,
		`CREATE TABLE role (id integer primary key, status integer not null default 1, deleted_at integer not null default 0)`,
		`CREATE TABLE admin_user_role_map (id integer primary key autoincrement, uid integer not null default 0, role_id integer not null default 0)`,
		`CREATE TABLE admin_user_department_map (id integer primary key autoincrement, uid integer not null default 0, dept_id integer not null default 0)`,
		`CREATE TABLE department_role_map (id integer primary key autoincrement, dept_id integer not null default 0, role_id integer not null default 0)`,
		`CREATE TABLE role_menu_map (id integer primary key autoincrement, role_id integer not null default 0, menu_id integer not null default 0)`,
		`CREATE TABLE menu (id integer primary key, code text not null default '', pids text not null default '', type integer not null default 1, is_auth integer not null default 0, status integer not null default 1, deleted_at integer not null default 0)`,
		`CREATE TABLE api (id integer primary key, route text not null default '', method text not null default '', is_auth integer not null default 2, is_effective integer not null default 1, deleted_at integer not null default 0)`,
		`CREATE TABLE menu_api_map (
			id integer primary key autoincrement,
			menu_id integer not null default 0,
			api_id integer not null default 0,
			is_default integer not null default 0,
			created_at datetime null,
			updated_at datetime null,
			unique(menu_id, api_id)
		)`,
	}
	for _, statement := range statements {
		execPermissionClosureSQL(t, db, statement)
	}
	return db
}

func execPermissionClosureSQL(t *testing.T, db *gorm.DB, statement string, args ...any) {
	t.Helper()
	if err := db.Exec(statement, args...).Error; err != nil {
		t.Fatalf("exec sql failed: %v\nsql: %s", err, statement)
	}
}

func assertPolicySet(t *testing.T, got [][]string, want []string) {
	t.Helper()

	gotKeys := make([]string, 0, len(got))
	for _, policy := range got {
		if len(policy) < 2 {
			t.Fatalf("invalid policy: %#v", policy)
		}
		gotKeys = append(gotKeys, policy[1]+" "+policy[0])
	}
	sort.Strings(gotKeys)
	sort.Strings(want)
	if !reflect.DeepEqual(gotKeys, want) {
		t.Fatalf("unexpected policies: got %v want %v", gotKeys, want)
	}
}
