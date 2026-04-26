package model

import (
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SysConfigI18n struct {
	BaseModel
	ConfigID   uint   `json:"config_id" gorm:"column:config_id;type:int unsigned;not null;default:0;index:uniq_config_id_locale,unique"`
	Locale     string `json:"locale" gorm:"column:locale;type:varchar(20);not null;default:'';index:uniq_config_id_locale,unique;index:idx_locale_config_name"`
	ConfigName string `json:"config_name" gorm:"column:config_name;type:varchar(100);not null;default:''"`
}

type SysDictTypeI18n struct {
	BaseModel
	DictTypeID uint   `json:"dict_type_id" gorm:"column:dict_type_id;type:int unsigned;not null;default:0;index:uniq_dict_type_id_locale,unique"`
	Locale     string `json:"locale" gorm:"column:locale;type:varchar(20);not null;default:'';index:uniq_dict_type_id_locale,unique;index:idx_locale_type_name"`
	TypeName   string `json:"type_name" gorm:"column:type_name;type:varchar(100);not null;default:''"`
}

type SysDictItemI18n struct {
	BaseModel
	DictItemID uint   `json:"dict_item_id" gorm:"column:dict_item_id;type:int unsigned;not null;default:0;index:uniq_dict_item_id_locale,unique"`
	Locale     string `json:"locale" gorm:"column:locale;type:varchar(20);not null;default:'';index:uniq_dict_item_id_locale,unique;index:idx_locale_label"`
	Label      string `json:"label" gorm:"column:label;type:varchar(100);not null;default:''"`
}

func NewSysConfigI18n() *SysConfigI18n {
	return BindModel(&SysConfigI18n{})
}

func NewSysDictTypeI18n() *SysDictTypeI18n {
	return BindModel(&SysDictTypeI18n{})
}

func NewSysDictItemI18n() *SysDictItemI18n {
	return BindModel(&SysDictItemI18n{})
}

func (m *SysConfigI18n) TableName() string {
	return "sys_config_i18n"
}

func (m *SysDictTypeI18n) TableName() string {
	return "sys_dict_type_i18n"
}

func (m *SysDictItemI18n) TableName() string {
	return "sys_dict_item_i18n"
}

func (m *SysConfigI18n) UpsertConfigNames(configID uint, localeNames map[string]string, tx ...*gorm.DB) error {
	if configID == 0 || len(localeNames) == 0 {
		return nil
	}
	if len(tx) > 0 && tx[0] != nil {
		m.SetDB(tx[0])
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	rows := make([]SysConfigI18n, 0, len(localeNames))
	for locale, name := range localeNames {
		trimmedLocale := strings.TrimSpace(locale)
		trimmedName := strings.TrimSpace(name)
		if trimmedLocale == "" || trimmedName == "" {
			continue
		}
		rows = append(rows, SysConfigI18n{
			ConfigID:   configID,
			Locale:     trimmedLocale,
			ConfigName: trimmedName,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "config_id"}, {Name: "locale"}},
		DoUpdates: clause.AssignmentColumns([]string{"config_name", "updated_at"}),
	}).Create(&rows).Error
}

func (m *SysDictTypeI18n) UpsertTypeNames(dictTypeID uint, localeNames map[string]string, tx ...*gorm.DB) error {
	if dictTypeID == 0 || len(localeNames) == 0 {
		return nil
	}
	if len(tx) > 0 && tx[0] != nil {
		m.SetDB(tx[0])
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	rows := make([]SysDictTypeI18n, 0, len(localeNames))
	for locale, name := range localeNames {
		trimmedLocale := strings.TrimSpace(locale)
		trimmedName := strings.TrimSpace(name)
		if trimmedLocale == "" || trimmedName == "" {
			continue
		}
		rows = append(rows, SysDictTypeI18n{
			DictTypeID: dictTypeID,
			Locale:     trimmedLocale,
			TypeName:   trimmedName,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dict_type_id"}, {Name: "locale"}},
		DoUpdates: clause.AssignmentColumns([]string{"type_name", "updated_at"}),
	}).Create(&rows).Error
}

func (m *SysDictItemI18n) UpsertLabels(dictItemID uint, localeLabels map[string]string, tx ...*gorm.DB) error {
	if dictItemID == 0 || len(localeLabels) == 0 {
		return nil
	}
	if len(tx) > 0 && tx[0] != nil {
		m.SetDB(tx[0])
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	rows := make([]SysDictItemI18n, 0, len(localeLabels))
	for locale, label := range localeLabels {
		trimmedLocale := strings.TrimSpace(locale)
		trimmedLabel := strings.TrimSpace(label)
		if trimmedLocale == "" || trimmedLabel == "" {
			continue
		}
		rows = append(rows, SysDictItemI18n{
			DictItemID: dictItemID,
			Locale:     trimmedLocale,
			Label:      trimmedLabel,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dict_item_id"}, {Name: "locale"}},
		DoUpdates: clause.AssignmentColumns([]string{"label", "updated_at"}),
	}).Create(&rows).Error
}

func (m *SysConfigI18n) LocalizedNameMapByConfigIDs(configIDs []uint, localePriority []string) (map[uint]string, error) {
	result := make(map[uint]string, len(configIDs))
	rows, priorities, err := m.listRowsByConfigIDs(configIDs, localePriority)
	if err != nil || len(rows) == 0 {
		return result, err
	}

	grouped := make(map[uint]map[string]string, len(configIDs))
	for _, row := range rows {
		if _, ok := grouped[row.ConfigID]; !ok {
			grouped[row.ConfigID] = make(map[string]string)
		}
		grouped[row.ConfigID][strings.TrimSpace(row.Locale)] = strings.TrimSpace(row.ConfigName)
	}
	for _, id := range configIDs {
		result[id] = pickLocalizedText(grouped[id], priorities)
	}
	return result, nil
}

func (m *SysConfigI18n) LocaleNameMapByConfigID(configID uint) (map[string]string, error) {
	result := make(map[string]string)
	if configID == 0 {
		return result, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []SysConfigI18n
	if err := db.Where("config_id = ?", configID).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		locale := strings.TrimSpace(row.Locale)
		name := strings.TrimSpace(row.ConfigName)
		if locale == "" || name == "" {
			continue
		}
		result[locale] = name
	}
	return result, nil
}

func (m *SysConfigI18n) DeleteByConfigIDs(configIDs []uint, tx ...*gorm.DB) error {
	if len(configIDs) == 0 {
		return nil
	}
	if len(tx) > 0 && tx[0] != nil {
		m.SetDB(tx[0])
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("config_id IN ?", configIDs).Delete(&SysConfigI18n{}).Error
}

func (m *SysDictTypeI18n) LocalizedNameMapByTypeIDs(typeIDs []uint, localePriority []string) (map[uint]string, error) {
	result := make(map[uint]string, len(typeIDs))
	rows, priorities, err := m.listRowsByTypeIDs(typeIDs, localePriority)
	if err != nil || len(rows) == 0 {
		return result, err
	}

	grouped := make(map[uint]map[string]string, len(typeIDs))
	for _, row := range rows {
		if _, ok := grouped[row.DictTypeID]; !ok {
			grouped[row.DictTypeID] = make(map[string]string)
		}
		grouped[row.DictTypeID][strings.TrimSpace(row.Locale)] = strings.TrimSpace(row.TypeName)
	}
	for _, id := range typeIDs {
		result[id] = pickLocalizedText(grouped[id], priorities)
	}
	return result, nil
}

func (m *SysDictTypeI18n) LocaleNameMapByTypeID(dictTypeID uint) (map[string]string, error) {
	result := make(map[string]string)
	if dictTypeID == 0 {
		return result, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []SysDictTypeI18n
	if err := db.Where("dict_type_id = ?", dictTypeID).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		locale := strings.TrimSpace(row.Locale)
		name := strings.TrimSpace(row.TypeName)
		if locale == "" || name == "" {
			continue
		}
		result[locale] = name
	}
	return result, nil
}

func (m *SysDictTypeI18n) DeleteByTypeIDs(dictTypeIDs []uint, tx ...*gorm.DB) error {
	if len(dictTypeIDs) == 0 {
		return nil
	}
	if len(tx) > 0 && tx[0] != nil {
		m.SetDB(tx[0])
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("dict_type_id IN ?", dictTypeIDs).Delete(&SysDictTypeI18n{}).Error
}

func (m *SysDictItemI18n) LocalizedLabelMapByItemIDs(itemIDs []uint, localePriority []string) (map[uint]string, error) {
	result := make(map[uint]string, len(itemIDs))
	rows, priorities, err := m.listRowsByItemIDs(itemIDs, localePriority)
	if err != nil || len(rows) == 0 {
		return result, err
	}

	grouped := make(map[uint]map[string]string, len(itemIDs))
	for _, row := range rows {
		if _, ok := grouped[row.DictItemID]; !ok {
			grouped[row.DictItemID] = make(map[string]string)
		}
		grouped[row.DictItemID][strings.TrimSpace(row.Locale)] = strings.TrimSpace(row.Label)
	}
	for _, id := range itemIDs {
		result[id] = pickLocalizedText(grouped[id], priorities)
	}
	return result, nil
}

func (m *SysDictItemI18n) LocaleLabelMapByItemID(dictItemID uint) (map[string]string, error) {
	result := make(map[string]string)
	if dictItemID == 0 {
		return result, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []SysDictItemI18n
	if err := db.Where("dict_item_id = ?", dictItemID).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		locale := strings.TrimSpace(row.Locale)
		label := strings.TrimSpace(row.Label)
		if locale == "" || label == "" {
			continue
		}
		result[locale] = label
	}
	return result, nil
}

func (m *SysDictItemI18n) DeleteByItemIDs(dictItemIDs []uint, tx ...*gorm.DB) error {
	if len(dictItemIDs) == 0 {
		return nil
	}
	if len(tx) > 0 && tx[0] != nil {
		m.SetDB(tx[0])
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("dict_item_id IN ?", dictItemIDs).Delete(&SysDictItemI18n{}).Error
}

func (m *SysConfigI18n) listRowsByConfigIDs(configIDs []uint, localePriority []string) ([]SysConfigI18n, []string, error) {
	priorities := normalizeLocalePriority(localePriority)
	if len(configIDs) == 0 || len(priorities) == 0 {
		return nil, priorities, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, priorities, err
	}
	var rows []SysConfigI18n
	if err := db.Where("config_id IN ? AND locale IN ?", configIDs, priorities).Find(&rows).Error; err != nil {
		return nil, priorities, err
	}
	return rows, priorities, nil
}

func (m *SysDictTypeI18n) listRowsByTypeIDs(typeIDs []uint, localePriority []string) ([]SysDictTypeI18n, []string, error) {
	priorities := normalizeLocalePriority(localePriority)
	if len(typeIDs) == 0 || len(priorities) == 0 {
		return nil, priorities, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, priorities, err
	}
	var rows []SysDictTypeI18n
	if err := db.Where("dict_type_id IN ? AND locale IN ?", typeIDs, priorities).Find(&rows).Error; err != nil {
		return nil, priorities, err
	}
	return rows, priorities, nil
}

func (m *SysDictItemI18n) listRowsByItemIDs(itemIDs []uint, localePriority []string) ([]SysDictItemI18n, []string, error) {
	priorities := normalizeLocalePriority(localePriority)
	if len(itemIDs) == 0 || len(priorities) == 0 {
		return nil, priorities, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, priorities, err
	}
	var rows []SysDictItemI18n
	if err := db.Where("dict_item_id IN ? AND locale IN ?", itemIDs, priorities).Find(&rows).Error; err != nil {
		return nil, priorities, err
	}
	return rows, priorities, nil
}

func normalizeLocalePriority(localePriority []string) []string {
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
	return priorities
}

func pickLocalizedText(localeText map[string]string, priorities []string) string {
	if len(localeText) == 0 {
		return ""
	}
	for _, locale := range priorities {
		if text := strings.TrimSpace(localeText[locale]); text != "" {
			return text
		}
	}

	keys := make([]string, 0, len(localeText))
	for key := range localeText {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if text := strings.TrimSpace(localeText[key]); text != "" {
			return text
		}
	}
	return ""
}
