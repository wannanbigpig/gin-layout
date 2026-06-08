package access

import (
	"net/http"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestNewMenuAPIDefaultsServiceUsesIsolatedDefaultBindings(t *testing.T) {
	first := NewMenuAPIDefaultsService()
	second := NewMenuAPIDefaultsService()
	if len(first.bindings) == 0 || len(second.bindings) == 0 {
		t.Fatal("expected default bindings to be initialized")
	}

	originalRoute := second.bindings[0].Route
	first.bindings[0].Route = "/mutated-by-test"

	if second.bindings[0].Route != originalRoute {
		t.Fatal("expected default bindings to be isolated per service instance")
	}
}

func TestNewMenuAPIDefaultsServiceWithDepsClonesBindings(t *testing.T) {
	customBindings := []defaultMenuAPIBinding{
		{MenuCode: "menu:test", Route: "/custom", Method: "GET"},
	}

	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		Bindings: customBindings,
	})
	customBindings[0].Route = "/changed"

	if service.bindings[0].Route != "/custom" {
		t.Fatal("expected custom bindings to be cloned")
	}
}

func TestNewMenuAPIDefaultsServiceWithDepsAllowsEmptyBindings(t *testing.T) {
	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		Bindings: []defaultMenuAPIBinding{},
	})

	if len(service.bindings) != 0 {
		t.Fatalf("expected empty bindings, got %d", len(service.bindings))
	}
}

func TestDefaultMenuAPIBindingsCoverManagementRoutes(t *testing.T) {
	service := NewMenuAPIDefaultsService()
	bindingSet := make(map[string]struct{}, len(service.bindings))
	for _, binding := range service.bindings {
		bindingSet[binding.Method+" "+binding.Route] = struct{}{}
	}

	required := []string{
		http.MethodGet + " /admin/v1/system/config/list",
		http.MethodGet + " /admin/v1/system/config/detail",
		http.MethodPost + " /admin/v1/system/config/create",
		http.MethodPost + " /admin/v1/system/config/update",
		http.MethodGet + " /admin/v1/admin-user/get-full-phone",
		http.MethodGet + " /admin/v1/admin-user/get-full-email",
		http.MethodPost + " /admin/v1/menu/update-all-menu-permissions",
		http.MethodGet + " /admin/v1/system/dict/type/list",
		http.MethodGet + " /admin/v1/system/dict/options",
		http.MethodGet + " /admin/v1/task/list",
		http.MethodPost + " /admin/v1/task/trigger",
		http.MethodPost + " /admin/v1/task/run/retry",
		http.MethodPost + " /admin/v1/task/run/cancel",
		http.MethodGet + " /admin/v1/system/export/list",
		http.MethodGet + " /admin/v1/system/export/detail",
		http.MethodPost + " /admin/v1/system/file/export",
		http.MethodPost + " /admin/v1/system/file/batch-delete",
		http.MethodPost + " /admin/v1/system/file/upload/credential/batch",
		http.MethodPost + " /admin/v1/system/file/upload/complete/batch",
		http.MethodGet + " /admin/v1/log/request/list",
		http.MethodGet + " /admin/v1/log/request/detail",
		http.MethodPost + " /admin/v1/log/request/mask-config",
	}
	for _, route := range required {
		if _, ok := bindingSet[route]; !ok {
			t.Fatalf("missing default menu API binding for %s", route)
		}
	}
}

func TestCollectMenuAPIBindingKeysKeepsUniqueValues(t *testing.T) {
	bindings := []defaultMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:update", Route: "/admin/v1/menu/update", Method: http.MethodPost},
	}

	menuCodes, routes, methods := collectMenuAPIBindingKeys(bindings)

	if len(menuCodes) != 2 || menuCodes[0] != "menu:list" || menuCodes[1] != "menu:update" {
		t.Fatalf("unexpected menu codes: %#v", menuCodes)
	}
	if len(routes) != 2 || routes[0] != "/admin/v1/menu/list" || routes[1] != "/admin/v1/menu/update" {
		t.Fatalf("unexpected routes: %#v", routes)
	}
	if len(methods) != 2 || methods[0] != http.MethodGet || methods[1] != http.MethodPost {
		t.Fatalf("unexpected methods: %#v", methods)
	}
}

func TestBuildDefaultMenuAPIMappingsSkipsMissingTargets(t *testing.T) {
	bindings := []defaultMenuAPIBinding{
		{MenuCode: "menu:list", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:missing", Route: "/admin/v1/menu/list", Method: http.MethodGet},
		{MenuCode: "menu:list", Route: "/admin/v1/menu/missing", Method: http.MethodGet},
	}
	targets := defaultMenuAPITargets{
		menuIDByCode: map[string]uint{
			"menu:list": 10,
		},
		apiIDByRouteMethod: map[string]uint{
			http.MethodGet + ":/admin/v1/menu/list": 20,
		},
	}

	mappings := buildDefaultMenuAPIMappings(bindings, targets)

	if len(mappings) != 1 {
		t.Fatalf("expected one mapping, got %d", len(mappings))
	}
	want := &model.MenuApiMap{MenuId: 10, ApiId: 20}
	if mappings[0].MenuId != want.MenuId || mappings[0].ApiId != want.ApiId {
		t.Fatalf("unexpected mapping: got=%+v want=%+v", mappings[0], want)
	}
}

func TestMenuAPIDefaultsSyncReconcilesDefaultRows(t *testing.T) {
	db := newMenuAPIDefaultsTestDB(t)
	execMenuAPITestSQL(t, db, "INSERT INTO menu (id, code, deleted_at) VALUES (1, 'old:button', 0), (2, 'new:button', 0), (3, 'manual:button', 0)")
	execMenuAPITestSQL(t, db, "INSERT INTO api (id, route, method, deleted_at) VALUES (10, '/old', 'GET', 0), (20, '/new', 'GET', 0), (30, '/manual', 'GET', 0)")
	execMenuAPITestSQL(t, db, "INSERT INTO menu_api_map (id, menu_id, api_id, is_default) VALUES (1, 1, 10, 1), (2, 3, 30, 0)")

	service := NewMenuAPIDefaultsServiceWithDeps(MenuAPIDefaultsServiceDeps{
		Bindings: []defaultMenuAPIBinding{
			{MenuCode: "new:button", Route: "/new", Method: http.MethodGet},
		},
	})

	if err := service.Sync(db); err != nil {
		t.Fatalf("sync default menu api mappings: %v", err)
	}

	type row struct {
		MenuID    uint `gorm:"column:menu_id"`
		APIID     uint `gorm:"column:api_id"`
		IsDefault int  `gorm:"column:is_default"`
	}
	var rows []row
	if err := db.Table("menu_api_map").Select("menu_id", "api_id", "is_default").Order("menu_id, api_id").Find(&rows).Error; err != nil {
		t.Fatalf("query mappings: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 mappings after reconcile, got %#v", rows)
	}
	if rows[0] != (row{MenuID: 2, APIID: 20, IsDefault: 1}) {
		t.Fatalf("expected new default mapping, got %#v", rows[0])
	}
	if rows[1] != (row{MenuID: 3, APIID: 30, IsDefault: 0}) {
		t.Fatalf("expected manual mapping to remain untouched, got %#v", rows[1])
	}
}

func newMenuAPIDefaultsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	statements := []string{
		`CREATE TABLE menu (id integer primary key, code text not null default '', deleted_at integer not null default 0)`,
		`CREATE TABLE api (id integer primary key, route text not null default '', method text not null default '', deleted_at integer not null default 0)`,
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
		execMenuAPITestSQL(t, db, statement)
	}
	return db
}

func execMenuAPITestSQL(t *testing.T, db *gorm.DB, statement string) {
	t.Helper()
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("exec sql failed: %v\nsql: %s", err, statement)
	}
}
