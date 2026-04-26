package sys_dict

import (
	"strings"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var sysDictTypeDiffRules = []auditdiff.FieldRule{
	{Field: "type_code", Label: "字典类型编码"},
	{Field: "type_name_i18n", Label: "字典类型名称"},
	{
		Field: "status",
		Label: "状态",
		ValueLabels: map[string]string{
			"0": "禁用",
			"1": "启用",
		},
	},
	{Field: "sort", Label: "排序"},
	{Field: "remark", Label: "备注"},
}

var sysDictItemDiffRules = []auditdiff.FieldRule{
	{Field: "type_code", Label: "字典类型编码"},
	{Field: "label_i18n", Label: "字典标签"},
	{Field: "value", Label: "字典值"},
	{Field: "color", Label: "颜色"},
	{Field: "tag_type", Label: "标签类型"},
	{
		Field: "is_default",
		Label: "默认项",
		ValueLabels: map[string]string{
			"0": "否",
			"1": "是",
		},
	},
	{
		Field: "status",
		Label: "状态",
		ValueLabels: map[string]string{
			"0": "禁用",
			"1": "启用",
		},
	},
	{Field: "sort", Label: "排序"},
	{Field: "remark", Label: "备注"},
}

// CreateTypeWithAuditDiff 创建字典类型并返回字段级 change_diff JSON。
func (s *SysDictService) CreateTypeWithAuditDiff(params *form.CreateSysDictType) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	if err := s.applyTypeMutation(0, &params.SysDictTypePayload); err != nil {
		return "", err
	}
	after, err := s.snapshotTypeByTypeCode(params.TypeCode)
	if err != nil {
		return "", nil
	}
	return buildSysDictTypeDiffJSON(nil, after), nil
}

// UpdateTypeWithAuditDiff 更新字典类型并返回字段级 change_diff JSON。
func (s *SysDictService) UpdateTypeWithAuditDiff(params *form.UpdateSysDictType) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotTypeByID(params.Id)
	if err != nil {
		return "", err
	}
	if err := s.applyTypeMutation(params.Id, &params.SysDictTypePayload); err != nil {
		return "", err
	}
	after, err := s.snapshotTypeByID(params.Id)
	if err != nil {
		return "", nil
	}
	return buildSysDictTypeDiffJSON(before, after), nil
}

// DeleteTypeWithAuditDiff 删除字典类型并返回字段级 change_diff JSON。
func (s *SysDictService) DeleteTypeWithAuditDiff(id uint) (string, error) {
	before, err := s.snapshotTypeByID(id)
	if err != nil {
		return "", err
	}
	if err := s.deleteType(id); err != nil {
		return "", err
	}
	return buildSysDictTypeDiffJSON(before, nil), nil
}

// CreateItemWithAuditDiff 创建字典项并返回字段级 change_diff JSON。
func (s *SysDictService) CreateItemWithAuditDiff(params *form.CreateSysDictItem) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	if err := s.applyItemMutation(0, &params.SysDictItemPayload); err != nil {
		return "", err
	}
	after, err := s.snapshotItemByTypeValue(params.TypeCode, params.Value)
	if err != nil {
		return "", nil
	}
	return buildSysDictItemDiffJSON(nil, after), nil
}

// UpdateItemWithAuditDiff 更新字典项并返回字段级 change_diff JSON。
func (s *SysDictService) UpdateItemWithAuditDiff(params *form.UpdateSysDictItem) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotItemByID(params.Id)
	if err != nil {
		return "", err
	}
	if err := s.applyItemMutation(params.Id, &params.SysDictItemPayload); err != nil {
		return "", err
	}
	after, err := s.snapshotItemByID(params.Id)
	if err != nil {
		return "", nil
	}
	return buildSysDictItemDiffJSON(before, after), nil
}

// DeleteItemWithAuditDiff 删除字典项并返回字段级 change_diff JSON。
func (s *SysDictService) DeleteItemWithAuditDiff(id uint) (string, error) {
	before, err := s.snapshotItemByID(id)
	if err != nil {
		return "", err
	}
	if err := s.deleteItem(id); err != nil {
		return "", err
	}
	return buildSysDictItemDiffJSON(before, nil), nil
}

func (s *SysDictService) deleteType(id uint) error {
	dictType := model.NewSysDictType()
	if err := dictType.GetById(id); err != nil || dictType.ID == 0 {
		return e.NewBusinessError(e.NotFound)
	}
	if dictType.IsProtected() {
		return e.NewBusinessError(e.InvalidParameter)
	}
	count, err := model.NewSysDictItem().CountByTypeCode(dictType.TypeCode)
	if err != nil {
		return err
	}
	if count > 0 {
		return e.NewBusinessError(e.InvalidParameter)
	}
	db, err := dictType.GetDB()
	if err != nil {
		return err
	}
	return access.RunInTransaction(db, func(tx *gorm.DB) error {
		dictType.SetDB(tx)
		if _, deleteErr := dictType.DeleteByID(id); deleteErr != nil {
			return deleteErr
		}
		return model.NewSysDictTypeI18n().DeleteByTypeIDs([]uint{id}, tx)
	})
}

func (s *SysDictService) deleteItem(id uint) error {
	item := model.NewSysDictItem()
	if err := item.GetById(id); err != nil || item.ID == 0 {
		return e.NewBusinessError(e.NotFound)
	}
	if item.IsProtected() {
		return e.NewBusinessError(e.InvalidParameter)
	}
	db, err := item.GetDB()
	if err != nil {
		return err
	}
	return access.RunInTransaction(db, func(tx *gorm.DB) error {
		item.SetDB(tx)
		if _, deleteErr := item.DeleteByID(id); deleteErr != nil {
			return deleteErr
		}
		return model.NewSysDictItemI18n().DeleteByItemIDs([]uint{id}, tx)
	})
}

func (s *SysDictService) snapshotTypeByID(id uint) (map[string]any, error) {
	dictType := model.NewSysDictType()
	if err := dictType.GetById(id); err != nil || dictType.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return snapshotDictType(dictType)
}

func (s *SysDictService) snapshotTypeByTypeCode(typeCode string) (map[string]any, error) {
	dictType := model.NewSysDictType()
	if err := dictType.FindByTypeCode(strings.TrimSpace(typeCode)); err != nil {
		return nil, err
	}
	return snapshotDictType(dictType)
}

func snapshotDictType(dictType *model.SysDictType) (map[string]any, error) {
	if dictType == nil || dictType.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	typeNameI18n, err := model.NewSysDictTypeI18n().LocaleNameMapByTypeID(dictType.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type_code":      dictType.TypeCode,
		"type_name_i18n": typeNameI18n,
		"status":         dictType.Status,
		"sort":           dictType.Sort,
		"remark":         dictType.Remark,
	}, nil
}

func (s *SysDictService) snapshotItemByID(id uint) (map[string]any, error) {
	item := model.NewSysDictItem()
	if err := item.GetById(id); err != nil || item.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return snapshotDictItem(item)
}

func (s *SysDictService) snapshotItemByTypeValue(typeCode, value string) (map[string]any, error) {
	item := model.NewSysDictItem()
	if err := item.FindByTypeCodeAndValue(strings.TrimSpace(typeCode), strings.TrimSpace(value)); err != nil {
		return nil, err
	}
	return snapshotDictItem(item)
}

func snapshotDictItem(item *model.SysDictItem) (map[string]any, error) {
	if item == nil || item.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	labelI18n, err := model.NewSysDictItemI18n().LocaleLabelMapByItemID(item.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type_code":  item.TypeCode,
		"label_i18n": labelI18n,
		"value":      item.Value,
		"color":      item.Color,
		"tag_type":   item.TagType,
		"is_default": item.IsDefault,
		"status":     item.Status,
		"sort":       item.Sort,
		"remark":     item.Remark,
	}, nil
}

func buildSysDictTypeDiffJSON(before, after map[string]any) string {
	return auditdiff.Marshal(auditdiff.BuildFieldDiff(before, after, sysDictTypeDiffRules))
}

func buildSysDictItemDiffJSON(before, after map[string]any) string {
	return auditdiff.Marshal(auditdiff.BuildFieldDiff(before, after, sysDictItemDiffRules))
}
