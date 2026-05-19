package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMenuI18nUpsertReplacesOmittedLocales(t *testing.T) {
	db := newI18nTestDB(t)
	if err := db.Create(&MenuI18n{MenuID: 1, Locale: "zh-CN", Title: "旧中文"}).Error; err != nil {
		t.Fatalf("create zh menu i18n failed: %v", err)
	}
	if err := db.Create(&MenuI18n{MenuID: 1, Locale: "en-US", Title: "Old English"}).Error; err != nil {
		t.Fatalf("create en menu i18n failed: %v", err)
	}

	if err := NewMenuI18n().UpsertMenuTitles(1, map[string]string{"zh-CN": "新中文"}, db); err != nil {
		t.Fatalf("upsert menu titles failed: %v", err)
	}

	var rows []MenuI18n
	if err := db.Where("menu_id = ?", 1).Find(&rows).Error; err != nil {
		t.Fatalf("query menu i18n failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Locale != "zh-CN" || rows[0].Title != "新中文" {
		t.Fatalf("expected menu i18n replace semantics, got %#v", rows)
	}
}

func TestMenuI18nUpsertSkipsEmptyLocaleOrTitle(t *testing.T) {
	db := newI18nTestDB(t)
	if err := db.Create(&MenuI18n{MenuID: 1, Locale: "zh-CN", Title: "保留中文"}).Error; err != nil {
		t.Fatalf("seed zh menu i18n failed: %v", err)
	}

	// 同时提交一个合法 locale 与若干空值组合，空值应被静默跳过，且不会触发 Replace 删除合法行。
	payload := map[string]string{
		"zh-CN":  "新中文",
		"":       "无 locale",
		"  ":     "纯空白 locale",
		"en-US":  "",
		"ja-JP":  "   ",
	}
	if err := NewMenuI18n().UpsertMenuTitles(1, payload, db); err != nil {
		t.Fatalf("upsert menu titles with empties failed: %v", err)
	}

	var rows []MenuI18n
	if err := db.Where("menu_id = ?", 1).Order("locale").Find(&rows).Error; err != nil {
		t.Fatalf("query menu i18n failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Locale != "zh-CN" || rows[0].Title != "新中文" {
		t.Fatalf("expected only zh-CN row retained, got %#v", rows)
	}
}

func TestSysI18nUpsertSkipsEmptyLocaleOrValue(t *testing.T) {
	db := newI18nTestDB(t)
	if err := db.Create(&SysConfigI18n{ConfigID: 1, Locale: "zh-CN", ConfigName: "旧配置"}).Error; err != nil {
		t.Fatalf("seed config i18n failed: %v", err)
	}
	if err := db.Create(&SysDictTypeI18n{DictTypeID: 2, Locale: "zh-CN", TypeName: "旧类型"}).Error; err != nil {
		t.Fatalf("seed dict type i18n failed: %v", err)
	}
	if err := db.Create(&SysDictItemI18n{DictItemID: 3, Locale: "zh-CN", Label: "旧选项"}).Error; err != nil {
		t.Fatalf("seed dict item i18n failed: %v", err)
	}

	// 全是非法行，UpsertXxx 应该全部跳过、什么都不写。
	emptyOnly := map[string]string{
		"":      "ignored",
		"  ":    "ignored2",
		"en-US": "",
		"ja-JP": "   ",
	}
	if err := NewSysConfigI18n().UpsertConfigNames(1, emptyOnly, db); err != nil {
		t.Fatalf("upsert config names with empties failed: %v", err)
	}
	if err := NewSysDictTypeI18n().UpsertTypeNames(2, emptyOnly, db); err != nil {
		t.Fatalf("upsert dict type names with empties failed: %v", err)
	}
	if err := NewSysDictItemI18n().UpsertLabels(3, emptyOnly, db); err != nil {
		t.Fatalf("upsert dict item labels with empties failed: %v", err)
	}

	assertConfigLocales(t, db, 1, map[string]string{"zh-CN": "旧配置"})
	assertDictTypeLocales(t, db, 2, map[string]string{"zh-CN": "旧类型"})
	assertDictItemLocales(t, db, 3, map[string]string{"zh-CN": "旧选项"})
}

func TestSysI18nUpsertPatchSemanticsOnlyEnglish(t *testing.T) {
	db := newI18nTestDB(t)
	if err := db.Create(&SysConfigI18n{ConfigID: 10, Locale: "zh-CN", ConfigName: "保留中文配置"}).Error; err != nil {
		t.Fatalf("seed config zh failed: %v", err)
	}
	if err := db.Create(&SysConfigI18n{ConfigID: 10, Locale: "en-US", ConfigName: "Old English Config"}).Error; err != nil {
		t.Fatalf("seed config en failed: %v", err)
	}
	if err := db.Create(&SysDictTypeI18n{DictTypeID: 20, Locale: "zh-CN", TypeName: "保留中文类型"}).Error; err != nil {
		t.Fatalf("seed dict type zh failed: %v", err)
	}
	if err := db.Create(&SysDictTypeI18n{DictTypeID: 20, Locale: "en-US", TypeName: "Old English Type"}).Error; err != nil {
		t.Fatalf("seed dict type en failed: %v", err)
	}
	if err := db.Create(&SysDictItemI18n{DictItemID: 30, Locale: "zh-CN", Label: "保留中文选项"}).Error; err != nil {
		t.Fatalf("seed dict item zh failed: %v", err)
	}
	if err := db.Create(&SysDictItemI18n{DictItemID: 30, Locale: "en-US", Label: "Old English Item"}).Error; err != nil {
		t.Fatalf("seed dict item en failed: %v", err)
	}

	// 仅提交 en-US，PATCH 语义下 zh-CN 行必须保留。
	if err := NewSysConfigI18n().UpsertConfigNames(10, map[string]string{"en-US": "New English Config"}, db); err != nil {
		t.Fatalf("upsert config names en-only failed: %v", err)
	}
	if err := NewSysDictTypeI18n().UpsertTypeNames(20, map[string]string{"en-US": "New English Type"}, db); err != nil {
		t.Fatalf("upsert dict type names en-only failed: %v", err)
	}
	if err := NewSysDictItemI18n().UpsertLabels(30, map[string]string{"en-US": "New English Item"}, db); err != nil {
		t.Fatalf("upsert dict item labels en-only failed: %v", err)
	}

	assertConfigLocales(t, db, 10, map[string]string{
		"zh-CN": "保留中文配置",
		"en-US": "New English Config",
	})
	assertDictTypeLocales(t, db, 20, map[string]string{
		"zh-CN": "保留中文类型",
		"en-US": "New English Type",
	})
	assertDictItemLocales(t, db, 30, map[string]string{
		"zh-CN": "保留中文选项",
		"en-US": "New English Item",
	})
}

func TestMenuI18nEmptyLocaleValueStillTriggersReplaceOnOmittedRows(t *testing.T) {
	db := newI18nTestDB(t)
	if err := db.Create(&MenuI18n{MenuID: 11, Locale: "zh-CN", Title: "旧中文"}).Error; err != nil {
		t.Fatalf("seed menu zh failed: %v", err)
	}
	if err := db.Create(&MenuI18n{MenuID: 11, Locale: "en-US", Title: "Old English"}).Error; err != nil {
		t.Fatalf("seed menu en failed: %v", err)
	}

	// en-US 虽然出现在 payload 中，但值为空，过滤后等价于“本次未提交该 locale”，Replace 语义下应删除旧行。
	if err := NewMenuI18n().UpsertMenuTitles(11, map[string]string{
		"zh-CN": "新中文",
		"en-US": "   ",
	}, db); err != nil {
		t.Fatalf("upsert menu titles failed: %v", err)
	}

	var rows []MenuI18n
	if err := db.Where("menu_id = ?", 11).Order("locale").Find(&rows).Error; err != nil {
		t.Fatalf("query menu i18n failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Locale != "zh-CN" || rows[0].Title != "新中文" {
		t.Fatalf("expected replace semantics to delete omitted en-US row, got %#v", rows)
	}
}

func TestSysI18nUpsertKeepsOmittedLocales(t *testing.T) {
	db := newI18nTestDB(t)
	if err := db.Create(&SysConfigI18n{ConfigID: 1, Locale: "zh-CN", ConfigName: "旧配置"}).Error; err != nil {
		t.Fatalf("create zh config i18n failed: %v", err)
	}
	if err := db.Create(&SysConfigI18n{ConfigID: 1, Locale: "en-US", ConfigName: "Old Config"}).Error; err != nil {
		t.Fatalf("create en config i18n failed: %v", err)
	}
	if err := db.Create(&SysDictTypeI18n{DictTypeID: 2, Locale: "zh-CN", TypeName: "旧类型"}).Error; err != nil {
		t.Fatalf("create zh dict type i18n failed: %v", err)
	}
	if err := db.Create(&SysDictTypeI18n{DictTypeID: 2, Locale: "en-US", TypeName: "Old Type"}).Error; err != nil {
		t.Fatalf("create en dict type i18n failed: %v", err)
	}
	if err := db.Create(&SysDictItemI18n{DictItemID: 3, Locale: "zh-CN", Label: "旧选项"}).Error; err != nil {
		t.Fatalf("create zh dict item i18n failed: %v", err)
	}
	if err := db.Create(&SysDictItemI18n{DictItemID: 3, Locale: "en-US", Label: "Old Item"}).Error; err != nil {
		t.Fatalf("create en dict item i18n failed: %v", err)
	}

	if err := NewSysConfigI18n().UpsertConfigNames(1, map[string]string{"zh-CN": "新配置"}, db); err != nil {
		t.Fatalf("upsert config names failed: %v", err)
	}
	if err := NewSysDictTypeI18n().UpsertTypeNames(2, map[string]string{"zh-CN": "新类型"}, db); err != nil {
		t.Fatalf("upsert dict type names failed: %v", err)
	}
	if err := NewSysDictItemI18n().UpsertLabels(3, map[string]string{"zh-CN": "新选项"}, db); err != nil {
		t.Fatalf("upsert dict item labels failed: %v", err)
	}

	assertConfigLocales(t, db, 1, map[string]string{"zh-CN": "新配置", "en-US": "Old Config"})
	assertDictTypeLocales(t, db, 2, map[string]string{"zh-CN": "新类型", "en-US": "Old Type"})
	assertDictItemLocales(t, db, 3, map[string]string{"zh-CN": "新选项", "en-US": "Old Item"})
}

func TestSysI18nEmptyValueDoesNotClearExistingLocale(t *testing.T) {
	db := newI18nTestDB(t)
	if err := db.Create(&SysConfigI18n{ConfigID: 101, Locale: "zh-CN", ConfigName: "保留中文配置"}).Error; err != nil {
		t.Fatalf("seed config zh failed: %v", err)
	}
	if err := db.Create(&SysConfigI18n{ConfigID: 101, Locale: "en-US", ConfigName: "Old English Config"}).Error; err != nil {
		t.Fatalf("seed config en failed: %v", err)
	}
	if err := db.Create(&SysDictTypeI18n{DictTypeID: 201, Locale: "zh-CN", TypeName: "保留中文类型"}).Error; err != nil {
		t.Fatalf("seed dict type zh failed: %v", err)
	}
	if err := db.Create(&SysDictTypeI18n{DictTypeID: 201, Locale: "en-US", TypeName: "Old English Type"}).Error; err != nil {
		t.Fatalf("seed dict type en failed: %v", err)
	}
	if err := db.Create(&SysDictItemI18n{DictItemID: 301, Locale: "zh-CN", Label: "保留中文选项"}).Error; err != nil {
		t.Fatalf("seed dict item zh failed: %v", err)
	}
	if err := db.Create(&SysDictItemI18n{DictItemID: 301, Locale: "en-US", Label: "Old English Item"}).Error; err != nil {
		t.Fatalf("seed dict item en failed: %v", err)
	}

	// zh-CN 传空字符串只会被 PATCH 语义跳过，不会清空旧值。
	if err := NewSysConfigI18n().UpsertConfigNames(101, map[string]string{
		"zh-CN": "   ",
		"en-US": "New English Config",
	}, db); err != nil {
		t.Fatalf("upsert config names failed: %v", err)
	}
	if err := NewSysDictTypeI18n().UpsertTypeNames(201, map[string]string{
		"zh-CN": "",
		"en-US": "New English Type",
	}, db); err != nil {
		t.Fatalf("upsert dict type names failed: %v", err)
	}
	if err := NewSysDictItemI18n().UpsertLabels(301, map[string]string{
		"zh-CN": " ",
		"en-US": "New English Item",
	}, db); err != nil {
		t.Fatalf("upsert dict item labels failed: %v", err)
	}

	assertConfigLocales(t, db, 101, map[string]string{
		"zh-CN": "保留中文配置",
		"en-US": "New English Config",
	})
	assertDictTypeLocales(t, db, 201, map[string]string{
		"zh-CN": "保留中文类型",
		"en-US": "New English Type",
	})
	assertDictItemLocales(t, db, 301, map[string]string{
		"zh-CN": "保留中文选项",
		"en-US": "New English Item",
	})
}

func assertConfigLocales(t *testing.T, db *gorm.DB, configID uint, expected map[string]string) {
	t.Helper()

	var rows []SysConfigI18n
	if err := db.Where("config_id = ?", configID).Find(&rows).Error; err != nil {
		t.Fatalf("query config i18n failed: %v", err)
	}
	if len(rows) != len(expected) {
		t.Fatalf("unexpected config i18n count: got=%d want=%d rows=%#v", len(rows), len(expected), rows)
	}
	for _, row := range rows {
		if expected[row.Locale] != row.ConfigName {
			t.Fatalf("unexpected config i18n row: %#v expected=%#v", row, expected)
		}
	}
}

func assertDictTypeLocales(t *testing.T, db *gorm.DB, dictTypeID uint, expected map[string]string) {
	t.Helper()

	var rows []SysDictTypeI18n
	if err := db.Where("dict_type_id = ?", dictTypeID).Find(&rows).Error; err != nil {
		t.Fatalf("query dict type i18n failed: %v", err)
	}
	if len(rows) != len(expected) {
		t.Fatalf("unexpected dict type i18n count: got=%d want=%d rows=%#v", len(rows), len(expected), rows)
	}
	for _, row := range rows {
		if expected[row.Locale] != row.TypeName {
			t.Fatalf("unexpected dict type i18n row: %#v expected=%#v", row, expected)
		}
	}
}

func assertDictItemLocales(t *testing.T, db *gorm.DB, dictItemID uint, expected map[string]string) {
	t.Helper()

	var rows []SysDictItemI18n
	if err := db.Where("dict_item_id = ?", dictItemID).Find(&rows).Error; err != nil {
		t.Fatalf("query dict item i18n failed: %v", err)
	}
	if len(rows) != len(expected) {
		t.Fatalf("unexpected dict item i18n count: got=%d want=%d rows=%#v", len(rows), len(expected), rows)
	}
	for _, row := range rows {
		if expected[row.Locale] != row.Label {
			t.Fatalf("unexpected dict item i18n row: %#v expected=%#v", row, expected)
		}
	}
}

func newI18nTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	statements := []string{
		`CREATE TABLE menu_i18n (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			menu_id integer not null default 0,
			locale text not null default '',
			title text not null default '',
			UNIQUE(menu_id, locale)
		)`,
		`CREATE TABLE sys_config_i18n (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			config_id integer not null default 0,
			locale text not null default '',
			config_name text not null default '',
			UNIQUE(config_id, locale)
		)`,
		`CREATE TABLE sys_dict_type_i18n (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			dict_type_id integer not null default 0,
			locale text not null default '',
			type_name text not null default '',
			UNIQUE(dict_type_id, locale)
		)`,
		`CREATE TABLE sys_dict_item_i18n (
			id integer primary key autoincrement,
			created_at datetime,
			updated_at datetime,
			dict_item_id integer not null default 0,
			locale text not null default '',
			label text not null default '',
			UNIQUE(dict_item_id, locale)
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create i18n test table failed: %v", err)
		}
	}
	return db
}
