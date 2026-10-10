package resources

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
)

func TestRequestLogSelfCollectionIncludesTerminalFields(t *testing.T) {
	collection := NewRequestLogTransformer(i18n.LocaleZhCN).ToSelfCollection(1, 10, 1, []*model.RequestLogs{{
		OS:      "Windows 11",
		Browser: "Edge",
	}})
	if len(collection.Data) != 1 {
		t.Fatalf("unexpected collection: %#v", collection)
	}
	item, ok := collection.Data[0].(*RequestLogSelfListResources)
	if !ok || item.OS != "Windows 11" || item.Browser != "Edge" {
		t.Fatalf("unexpected terminal fields: %#v", collection.Data[0])
	}
}

func TestRequestLogSelfChangeSummarySupportsProductLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		changeDiff string
		want       []RequestLogSelfChangeItemResources
		omitted    int
	}{
		{
			name: "create",
			path: "/admin/v1/product/create",
			changeDiff: `[
				{"field":"id","label":"forged","after":12},
				{"field":"name","label":"forged","after":"Notebook"},
				{"field":"status","label":"forged","after":1,"after_display":"forged enabled"},
				{"field":"secret","label":"Secret","after":"must-not-leak"}
			]`,
			want: []RequestLogSelfChangeItemResources{
				{Label: "产品ID", ChangeType: selfChangeTypeCreated, AfterText: "12"},
				{Label: "产品名称", ChangeType: selfChangeTypeCreated, AfterText: "Notebook"},
				{Label: "状态", ChangeType: selfChangeTypeCreated, AfterText: "启用"},
			},
			omitted: 1,
		},
		{
			name: "update",
			path: "/admin/v1/product/update",
			changeDiff: `[
				{"field":"name","before":"Old","after":"New"},
				{"field":"price","before":100,"after":250},
				{"field":"status","before":1,"after":0,"before_display":"启用","after_display":"禁用"}
			]`,
			want: []RequestLogSelfChangeItemResources{
				{Label: "产品名称", ChangeType: selfChangeTypeUpdated, BeforeText: "Old", AfterText: "New"},
				{Label: "价格（分）", ChangeType: selfChangeTypeUpdated, BeforeText: "100", AfterText: "250"},
				{Label: "状态", ChangeType: selfChangeTypeUpdated, BeforeText: "启用", AfterText: "禁用"},
			},
		},
		{
			name: "delete",
			path: "/admin/v1/product/delete",
			changeDiff: `[
				{"field":"id","before":12},
				{"field":"name","before":"Notebook"},
				{"field":"status","before":1,"before_display":"启用"}
			]`,
			want: []RequestLogSelfChangeItemResources{
				{Label: "产品ID", ChangeType: selfChangeTypeRemoved, BeforeText: "12"},
				{Label: "产品名称", ChangeType: selfChangeTypeRemoved, BeforeText: "Notebook"},
				{Label: "状态", ChangeType: selfChangeTypeRemoved, BeforeText: "启用"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detail := NewRequestLogTransformer(i18n.LocaleZhCN).ToSelfDetail(&model.RequestLogs{
				BaseURL:    test.path,
				ChangeDiff: test.changeDiff,
			})
			if detail.ChangeSummary == nil {
				t.Fatal("expected change summary")
			}
			if got := detail.ChangeSummary.Items; !requestLogSelfChangeItemsEqual(got, test.want) {
				t.Fatalf("items = %#v, want %#v", got, test.want)
			}
			if detail.ChangeSummary.OmittedCount != test.omitted {
				t.Fatalf("omitted_count = %d, want %d", detail.ChangeSummary.OmittedCount, test.omitted)
			}
			assertRequestLogSelfDetailDoesNotContain(t, detail, "forged", "must-not-leak", "Secret")
		})
	}
}

func TestRequestLogSelfChangeSummarySupportsFileAndFolderChanges(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		changeDiff string
		want       []RequestLogSelfChangeItemResources
	}{
		{
			name: "rename file",
			path: "/admin/v1/system/file/update",
			changeDiff: `{
				"before":{"action":"update_file","value":[{"id":7,"origin_name":"old.txt","folder_id":1,"logical_path":"/docs"}]},
				"after":{"action":"update_file","value":[{"id":7,"origin_name":"new.txt","folder_id":1,"logical_path":"/docs","secret":"must-not-leak"}]}
			}`,
			want: []RequestLogSelfChangeItemResources{
				{Label: "文件名称", ChangeType: selfChangeTypeUpdated, BeforeText: "old.txt", AfterText: "new.txt"},
			},
		},
		{
			name: "move files",
			path: "/admin/v1/system/file/move",
			changeDiff: `{
				"before":{"action":"move_files","value":[{"id":7,"origin_name":"a.txt","folder_id":1,"logical_path":"/old"}]},
				"after":{"action":"move_files","value":{"files":[{"id":7,"origin_name":"a.txt","folder_id":2,"logical_path":"/new"}],"result":{"total":1,"moved":1}}}
			}`,
			want: []RequestLogSelfChangeItemResources{
				{Label: "文件所在目录ID", ChangeType: selfChangeTypeUpdated, BeforeText: "1", AfterText: "2"},
				{Label: "文件路径", ChangeType: selfChangeTypeUpdated, BeforeText: "/old", AfterText: "/new"},
			},
		},
		{
			name: "rename folder",
			path: "/admin/v1/system/file/folder/update",
			changeDiff: `{
				"before":{"action":"update_file_folder","value":{"id":3,"parent_id":1,"name":"old","logical_path":"/old"}},
				"after":{"action":"update_file_folder","value":{"id":3,"parent_id":1,"name":"new","logical_path":"/new"}}
			}`,
			want: []RequestLogSelfChangeItemResources{
				{Label: "目录名称", ChangeType: selfChangeTypeUpdated, BeforeText: "old", AfterText: "new"},
				{Label: "目录路径", ChangeType: selfChangeTypeUpdated, BeforeText: "/old", AfterText: "/new"},
			},
		},
		{
			name: "move folder",
			path: "/admin/v1/system/file/folder/move",
			changeDiff: `{
				"before":{"action":"move_file_folder","value":{"id":3,"parent_id":1,"name":"docs","logical_path":"/old/docs"}},
				"after":{"action":"move_file_folder","value":{"id":3,"parent_id":2,"name":"docs","logical_path":"/new/docs"}}
			}`,
			want: []RequestLogSelfChangeItemResources{
				{Label: "上级目录ID", ChangeType: selfChangeTypeUpdated, BeforeText: "1", AfterText: "2"},
				{Label: "目录路径", ChangeType: selfChangeTypeUpdated, BeforeText: "/old/docs", AfterText: "/new/docs"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detail := NewRequestLogTransformer(i18n.LocaleZhCN).ToSelfDetail(&model.RequestLogs{
				BaseURL:    test.path,
				ChangeDiff: test.changeDiff,
			})
			if detail.ChangeSummary == nil {
				t.Fatal("expected change summary")
			}
			if got := detail.ChangeSummary.Items; !requestLogSelfChangeItemsEqual(got, test.want) {
				t.Fatalf("items = %#v, want %#v", got, test.want)
			}
			assertRequestLogSelfDetailDoesNotContain(t, detail, "must-not-leak", "secret")
		})
	}
}

func TestRequestLogSelfChangeSummarySupportsFileActions(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		changeDiff string
		wantLabel  string
	}{
		{
			name:       "delete file",
			path:       "/admin/v1/system/file/delete",
			changeDiff: `{"before":null,"after":{"action":"delete_file","result":{"id":8,"deleted_reason":"private note"}}}`,
			wantLabel:  "已删除文件（ID 8）",
		},
		{
			name:       "restore file",
			path:       "/admin/v1/system/file/trash/restore",
			changeDiff: `{"before":null,"after":{"action":"restore_file","result":{"id":8,"private":"must-not-leak"}}}`,
			wantLabel:  "已恢复文件（ID 8）",
		},
		{
			name:       "delete folder",
			path:       "/admin/v1/system/file/folder/delete",
			changeDiff: `{"before":null,"after":{"action":"delete_file_folder","result":{"id":4,"deleted_reason":"private note"}}}`,
			wantLabel:  "已删除目录（ID 4）",
		},
		{
			name:       "batch restore files",
			path:       "/admin/v1/system/file/trash/batch-restore",
			changeDiff: `{"before":null,"after":{"action":"batch_restore_files","result":{"ids":[8,9],"private":"must-not-leak"}}}`,
			wantLabel:  "已恢复 2 个文件",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detail := NewRequestLogTransformer(i18n.LocaleZhCN).ToSelfDetail(&model.RequestLogs{
				BaseURL:    test.path,
				ChangeDiff: test.changeDiff,
			})
			if detail.ChangeSummary == nil || len(detail.ChangeSummary.Items) != 1 {
				t.Fatalf("unexpected change summary: %#v", detail.ChangeSummary)
			}
			item := detail.ChangeSummary.Items[0]
			if item.ChangeType != selfChangeTypeAction || item.Label != test.wantLabel {
				t.Fatalf("item = %#v, want action %q", item, test.wantLabel)
			}
			assertRequestLogSelfDetailDoesNotContain(t, detail, "private note", "must-not-leak", "deleted_reason")
		})
	}
}

func TestRequestLogSelfChangeSummaryRejectsMismatchedFileAction(t *testing.T) {
	detail := NewRequestLogTransformer(i18n.LocaleZhCN).ToSelfDetail(&model.RequestLogs{
		BaseURL: "/admin/v1/system/file/delete",
		ChangeDiff: `{"before":null,"after":{"action":"restore_file","result":{` +
			`"id":8,"secret":"must-not-leak"}}}`,
	})
	if detail.ChangeSummary == nil || len(detail.ChangeSummary.Items) != 0 || detail.ChangeSummary.OmittedCount != 1 {
		t.Fatalf("unexpected mismatched-action summary: %#v", detail.ChangeSummary)
	}
	assertRequestLogSelfDetailDoesNotContain(t, detail, "must-not-leak", "secret")
}

func TestRequestLogSelfChangeSummaryLocalizesProductAndFileLabels(t *testing.T) {
	product := NewRequestLogTransformer(i18n.LocaleEnUS).ToSelfDetail(&model.RequestLogs{
		BaseURL: "/admin/v1/product/update",
		ChangeDiff: `[{"field":"status","before":0,"after":1,` +
			`"before_display":"禁用","after_display":"启用"}]`,
	})
	if product.ChangeSummary == nil || len(product.ChangeSummary.Items) != 1 {
		t.Fatalf("unexpected product summary: %#v", product.ChangeSummary)
	}
	if got := product.ChangeSummary.Items[0]; got.Label != "Status" || got.BeforeText != "Disabled" || got.AfterText != "Enabled" {
		t.Fatalf("unexpected localized product item: %#v", got)
	}

	file := NewRequestLogTransformer(i18n.LocaleEnUS).ToSelfDetail(&model.RequestLogs{
		BaseURL:    "/admin/v1/system/file/delete",
		ChangeDiff: `{"before":null,"after":{"action":"delete_file","result":{"id":8}}}`,
	})
	if file.ChangeSummary == nil || len(file.ChangeSummary.Items) != 1 || file.ChangeSummary.Items[0].Label != "Deleted file (ID 8)" {
		t.Fatalf("unexpected localized file action: %#v", file.ChangeSummary)
	}
}

func TestRequestLogSelfChangeSummaryDescribesMenuPermissionRefresh(t *testing.T) {
	changeDiff := `[{"field":"action","after":"sync_all_menu_permissions"}]`
	for _, test := range []struct {
		locale string
		label  string
	}{
		{i18n.LocaleZhCN, "已刷新菜单权限缓存"},
		{i18n.LocaleEnUS, "Menu permission cache refreshed"},
	} {
		detail := NewRequestLogTransformer(test.locale).ToSelfDetail(&model.RequestLogs{
			BaseURL:    "/admin/v1/menu/update-all-menu-permissions",
			ChangeDiff: changeDiff,
		})
		if detail.ChangeSummary == nil || detail.ChangeSummary.OmittedCount != 0 || len(detail.ChangeSummary.Items) != 1 {
			t.Fatalf("unexpected summary for %s: %#v", test.locale, detail.ChangeSummary)
		}
		if item := detail.ChangeSummary.Items[0]; item.ChangeType != selfChangeTypeAction || item.Label != test.label {
			t.Fatalf("unexpected action for %s: %#v", test.locale, item)
		}
	}
	unknown := NewRequestLogTransformer(i18n.LocaleZhCN).ToSelfDetail(&model.RequestLogs{
		BaseURL:    "/admin/v1/menu/update-all-menu-permissions",
		ChangeDiff: `[{"field":"action","after":"unknown_action"}]`,
	})
	if unknown.ChangeSummary == nil || len(unknown.ChangeSummary.Items) != 0 || unknown.ChangeSummary.OmittedCount != 1 {
		t.Fatalf("unknown action must not be described as a menu refresh: %#v", unknown.ChangeSummary)
	}
}

func TestRequestLogSelfChangeSummarySupportsCoreManagementAndRedactsSensitiveDetails(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		changeDiff string
		wantLabel  string
		forbidden  []string
	}{
		{"user", "/admin/v1/admin-user/update", `[{"field":"nickname","before":"旧昵称","after":"新昵称"},{"field":"role_ids","before":[1],"after":[2]}]`, "昵称", []string{"role_ids", "[1]", "[2]"}},
		{"role", "/admin/v1/role/update", `[{"field":"name","before":"旧角色","after":"新角色"},{"field":"menu_list","before":[1],"after":[2]}]`, "角色名称", []string{"menu_list", "[1]", "[2]"}},
		{"department", "/admin/v1/department/update", `[{"field":"name","before":"旧部门","after":"新部门"},{"field":"role_ids","before":[1],"after":[2]}]`, "部门名称", []string{"role_ids", "[1]", "[2]"}},
		{"menu", "/admin/v1/menu/update", `[{"field":"name","before":"oldRoute","after":"newRoute"},{"field":"api_list","before":[11],"after":[12]}]`, "路由名称", []string{"api_list", "[11]", "[12]"}},
		{"dictionary", "/admin/v1/system/dict/item/update", `[{"field":"value","before":"draft","after":"published"},{"field":"label_i18n","before":{"zh-CN":"草稿"},"after":{"zh-CN":"已发布"}}]`, "字典值", []string{"label_i18n", "草稿", "已发布"}},
		{"config", "/admin/v1/system/config/update", `[{"field":"config_key","before":"site.name","after":"site.title"},{"field":"config_value","before":"secret-before","after":"secret-after"}]`, "参数键名", []string{"secret-before", "secret-after", "config_value"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detail := NewRequestLogTransformer(i18n.LocaleZhCN).ToSelfDetail(&model.RequestLogs{BaseURL: test.path, ChangeDiff: test.changeDiff})
			if detail.ChangeSummary == nil || len(detail.ChangeSummary.Items) != 2 {
				t.Fatalf("unexpected summary: %#v", detail.ChangeSummary)
			}
			if detail.ChangeSummary.Items[0].Label != test.wantLabel {
				t.Fatalf("first label = %q, want %q", detail.ChangeSummary.Items[0].Label, test.wantLabel)
			}
			if detail.ChangeSummary.Items[1].ChangeType != selfChangeTypeChanged || detail.ChangeSummary.Items[1].BeforeText != "" || detail.ChangeSummary.Items[1].AfterText != "" {
				t.Fatalf("sensitive/permission detail must only report changed: %#v", detail.ChangeSummary.Items[1])
			}
			assertRequestLogSelfDetailDoesNotContain(t, detail, test.forbidden...)
		})
	}
}

func requestLogSelfChangeItemsEqual(got, want []RequestLogSelfChangeItemResources) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func assertRequestLogSelfDetailDoesNotContain(t *testing.T, detail *RequestLogSelfDetailResources, forbidden ...string) {
	t.Helper()
	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal self detail: %v", err)
	}
	for _, value := range forbidden {
		if strings.Contains(string(raw), value) {
			t.Fatalf("self detail leaked %q: %s", value, raw)
		}
	}
}
