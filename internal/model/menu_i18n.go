package model

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MenuI18n 菜单标题多语言表。
type MenuI18n struct {
	BaseModel
	MenuID uint   `json:"menu_id" gorm:"column:menu_id;type:int unsigned;not null;default:0;index:uniq_menu_id_locale,unique;index:idx_locale_menu_id"`
	Locale string `json:"locale" gorm:"column:locale;type:varchar(10);not null;default:'';index:uniq_menu_id_locale,unique;index:idx_locale_menu_id"`
	Title  string `json:"title" gorm:"column:title;type:varchar(60);not null;default:''"`
}

func NewMenuI18n() *MenuI18n {
	return BindModel(&MenuI18n{})
}

// TableName 获取表名。
func (m *MenuI18n) TableName() string {
	return "menu_i18n"
}

// UpsertMenuTitles 按 menu_id + locale 幂等写入标题翻译。
func (m *MenuI18n) UpsertMenuTitles(menuID uint, localeTitles map[string]string, tx ...*gorm.DB) error {
	if menuID == 0 || len(localeTitles) == 0 {
		return nil
	}
	if len(tx) > 0 && tx[0] != nil {
		m.SetDB(tx[0])
	}

	db, err := m.GetDB()
	if err != nil {
		return err
	}

	rows := make([]MenuI18n, 0, len(localeTitles))
	for locale, title := range localeTitles {
		trimmedLocale := strings.TrimSpace(locale)
		trimmedTitle := strings.TrimSpace(title)
		if trimmedLocale == "" || trimmedTitle == "" {
			continue
		}
		rows = append(rows, MenuI18n{
			MenuID: menuID,
			Locale: trimmedLocale,
			Title:  trimmedTitle,
		})
	}
	if len(rows) == 0 {
		return nil
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "menu_id"}, {Name: "locale"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "updated_at"}),
	}).Create(&rows).Error
}

// LocalizedTitleMapByMenuIDs 按语言优先级批量查询菜单标题。
func (m *MenuI18n) LocalizedTitleMapByMenuIDs(menuIDs []uint, localePriority []string) (map[uint]string, error) {
	result := make(map[uint]string, len(menuIDs))
	if len(menuIDs) == 0 {
		return result, nil
	}

	priorities := make([]string, 0, len(localePriority))
	seen := make(map[string]struct{}, len(localePriority))
	for _, locale := range localePriority {
		trimmedLocale := strings.TrimSpace(locale)
		if trimmedLocale == "" {
			continue
		}
		if _, ok := seen[trimmedLocale]; ok {
			continue
		}
		seen[trimmedLocale] = struct{}{}
		priorities = append(priorities, trimmedLocale)
	}
	if len(priorities) == 0 {
		return result, nil
	}

	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}

	var rows []MenuI18n
	if err := db.Where("menu_id IN ? AND locale IN ?", menuIDs, priorities).Find(&rows).Error; err != nil {
		return nil, err
	}

	grouped := make(map[uint]map[string]string, len(menuIDs))
	for _, row := range rows {
		if _, ok := grouped[row.MenuID]; !ok {
			grouped[row.MenuID] = make(map[string]string)
		}
		grouped[row.MenuID][strings.TrimSpace(row.Locale)] = strings.TrimSpace(row.Title)
	}

	for _, menuID := range menuIDs {
		localizedMap := grouped[menuID]
		if len(localizedMap) == 0 {
			continue
		}
		for _, locale := range priorities {
			if title := strings.TrimSpace(localizedMap[locale]); title != "" {
				result[menuID] = title
				break
			}
		}
	}
	return result, nil
}

// LocaleTitleMapByMenuID 查询指定菜单的全部翻译。
func (m *MenuI18n) LocaleTitleMapByMenuID(menuID uint) (map[string]string, error) {
	result := make(map[string]string)
	if menuID == 0 {
		return result, nil
	}

	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}

	var rows []MenuI18n
	if err := db.Where("menu_id = ?", menuID).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		trimmedLocale := strings.TrimSpace(row.Locale)
		trimmedTitle := strings.TrimSpace(row.Title)
		if trimmedLocale == "" || trimmedTitle == "" {
			continue
		}
		result[trimmedLocale] = trimmedTitle
	}
	return result, nil
}

// DeleteByMenuIDs 删除菜单对应的翻译数据。
func (m *MenuI18n) DeleteByMenuIDs(menuIDs []uint, tx ...*gorm.DB) error {
	if len(menuIDs) == 0 {
		return nil
	}
	if len(tx) > 0 && tx[0] != nil {
		m.SetDB(tx[0])
	}

	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("menu_id IN ?", menuIDs).Delete(&MenuI18n{}).Error
}
