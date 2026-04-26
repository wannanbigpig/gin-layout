package sys_dict

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// SysDictService 系统字典服务。
type SysDictService struct {
	service.Base
}

func NewSysDictService() *SysDictService {
	return &SysDictService{}
}

func (s *SysDictService) TypeList(params *form.SysDictTypeList, locale string) *resources.Collection {
	condition, args := s.buildTypeCondition(params)
	total, collection, err := model.ListPageE(model.NewSysDictType(), params.Page, params.PerPage, condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	if err != nil {
		return resources.NewSysDictTypeTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}
	s.fillLocalizedTypeNames(collection, locale)
	return resources.NewSysDictTypeTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

func (s *SysDictService) TypeDetail(id uint, locale string) (any, error) {
	dictType := model.NewSysDictType()
	if err := dictType.GetById(id); err != nil || dictType.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	translations, err := model.NewSysDictTypeI18n().LocaleNameMapByTypeID(dictType.ID)
	if err != nil {
		return nil, err
	}
	dictType.TypeNameI18n = translations
	dictType.TypeName = service.ResolveLocaleText(translations, locale)
	return resources.NewSysDictTypeTransformer().ToStruct(dictType), nil
}

func (s *SysDictService) CreateType(params *form.CreateSysDictType) error {
	_, err := s.CreateTypeWithAuditDiff(params)
	return err
}

func (s *SysDictService) UpdateType(params *form.UpdateSysDictType) error {
	_, err := s.UpdateTypeWithAuditDiff(params)
	return err
}

func (s *SysDictService) DeleteType(id uint) error {
	_, err := s.DeleteTypeWithAuditDiff(id)
	return err
}

func (s *SysDictService) ItemList(params *form.SysDictItemList, locale string) *resources.Collection {
	condition, args := s.buildItemCondition(params)
	total, collection, err := model.ListPageE(model.NewSysDictItem(), params.Page, params.PerPage, condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id asc",
	})
	if err != nil {
		return resources.NewSysDictItemTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}
	s.fillLocalizedItemLabels(collection, locale)
	return resources.NewSysDictItemTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

func (s *SysDictService) CreateItem(params *form.CreateSysDictItem) error {
	_, err := s.CreateItemWithAuditDiff(params)
	return err
}

func (s *SysDictService) UpdateItem(params *form.UpdateSysDictItem) error {
	_, err := s.UpdateItemWithAuditDiff(params)
	return err
}

func (s *SysDictService) DeleteItem(id uint) error {
	_, err := s.DeleteItemWithAuditDiff(id)
	return err
}

func (s *SysDictService) Options(typeCode string, locale string) ([]resources.SysDictOptionResources, error) {
	typeCode = strings.TrimSpace(typeCode)
	if typeCode == "" {
		return nil, e.NewBusinessError(e.InvalidParameter)
	}
	if err := s.ensureEnabledType(typeCode); err != nil {
		return nil, err
	}
	items, err := model.NewSysDictItem().EnabledItemsByTypeCode(typeCode)
	if err != nil {
		return nil, err
	}
	s.fillLocalizedItemLabelsFromSlice(items, locale)
	return resources.ToSysDictOptions(items), nil
}

func (s *SysDictService) applyTypeMutation(id uint, params *form.SysDictTypePayload) error {
	params.TypeCode = strings.TrimSpace(params.TypeCode)
	params.TypeNameI18n = service.NormalizeLocaleTextMap(params.TypeNameI18n)
	params.Remark = strings.TrimSpace(params.Remark)
	if len(params.TypeNameI18n) == 0 {
		return e.NewBusinessError(e.InvalidParameter)
	}

	dictType := model.NewSysDictType()
	if id > 0 {
		if err := dictType.GetById(id); err != nil || dictType.ID == 0 {
			return e.NewBusinessError(e.NotFound)
		}
		if dictType.IsProtected() && dictType.TypeCode != params.TypeCode {
			return e.NewBusinessError(e.InvalidParameter)
		}
	}
	if exists, err := model.NewSysDictType().ExistsByTypeCodeExcludeID(params.TypeCode, id); err != nil {
		return err
	} else if exists {
		return e.NewBusinessError(e.InvalidParameter)
	}

	dictType.TypeCode = params.TypeCode
	dictType.Status = valueOrDefault(params.Status, defaultStatus(dictType.Status, id))
	dictType.Sort = params.Sort
	dictType.Remark = params.Remark
	db, err := dictType.GetDB()
	if err != nil {
		return err
	}
	return access.RunInTransaction(db, func(tx *gorm.DB) error {
		dictType.SetDB(tx)
		if saveErr := dictType.Save(); saveErr != nil {
			return saveErr
		}
		return model.NewSysDictTypeI18n().UpsertTypeNames(dictType.ID, params.TypeNameI18n, tx)
	})
}

func (s *SysDictService) applyItemMutation(id uint, params *form.SysDictItemPayload) error {
	params.TypeCode = strings.TrimSpace(params.TypeCode)
	params.LabelI18n = service.NormalizeLocaleTextMap(params.LabelI18n)
	params.Value = strings.TrimSpace(params.Value)
	params.Color = strings.TrimSpace(params.Color)
	params.TagType = strings.TrimSpace(params.TagType)
	params.Remark = strings.TrimSpace(params.Remark)
	if len(params.LabelI18n) == 0 {
		return e.NewBusinessError(e.InvalidParameter)
	}
	if err := s.ensureTypeExists(params.TypeCode); err != nil {
		return err
	}

	item := model.NewSysDictItem()
	if id > 0 {
		if err := item.GetById(id); err != nil || item.ID == 0 {
			return e.NewBusinessError(e.NotFound)
		}
		if item.IsProtected() && (item.TypeCode != params.TypeCode || item.Value != params.Value) {
			return e.NewBusinessError(e.InvalidParameter)
		}
	}
	if exists, err := model.NewSysDictItem().ExistsByValueExcludeID(params.TypeCode, params.Value, id); err != nil {
		return err
	} else if exists {
		return e.NewBusinessError(e.InvalidParameter)
	}

	item.TypeCode = params.TypeCode
	item.Value = params.Value
	item.Color = params.Color
	item.TagType = params.TagType
	item.IsDefault = valueOrDefault(params.IsDefault, item.IsDefault)
	item.Status = valueOrDefault(params.Status, defaultStatus(item.Status, id))
	item.Sort = params.Sort
	item.Remark = params.Remark

	db, err := item.GetDB()
	if err != nil {
		return err
	}
	return access.RunInTransaction(db, func(tx *gorm.DB) error {
		item.SetDB(tx)
		if item.IsDefault == 1 {
			if err := tx.Model(model.NewSysDictItem()).
				Where("type_code = ? AND id <> ? AND deleted_at = 0", item.TypeCode, item.ID).
				Update("is_default", 0).Error; err != nil {
				return err
			}
		}
		if err := item.Save(); err != nil {
			return err
		}
		return model.NewSysDictItemI18n().UpsertLabels(item.ID, params.LabelI18n, tx)
	})
}

func (s *SysDictService) ensureTypeExists(typeCode string) error {
	dictType := model.NewSysDictType()
	if err := dictType.FindByTypeCode(typeCode); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return e.NewBusinessError(e.InvalidParameter)
		}
		return err
	}
	return nil
}

func (s *SysDictService) ensureEnabledType(typeCode string) error {
	dictType := model.NewSysDictType()
	if err := dictType.FindByTypeCode(typeCode); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return e.NewBusinessError(e.NotFound)
		}
		return err
	}
	if dictType.Status != 1 {
		return e.NewBusinessError(e.NotFound)
	}
	return nil
}

func (s *SysDictService) buildTypeCondition(params *form.SysDictTypeList) (string, []any) {
	qb := query_builder.New().
		AddLike("type_code", params.TypeCode).
		AddEq("status", params.Status)
	if keyword := strings.TrimSpace(params.TypeName); keyword != "" {
		qb.AddCondition("id IN (SELECT dict_type_id FROM sys_dict_type_i18n WHERE type_name like ?)", "%"+keyword+"%")
	}
	return qb.Build()
}

func (s *SysDictService) buildItemCondition(params *form.SysDictItemList) (string, []any) {
	qb := query_builder.New().
		AddEq("type_code", params.TypeCode).
		AddLike("value", params.Value).
		AddEq("status", params.Status)
	if keyword := strings.TrimSpace(params.Label); keyword != "" {
		qb.AddCondition("id IN (SELECT dict_item_id FROM sys_dict_item_i18n WHERE label like ?)", "%"+keyword+"%")
	}
	return qb.Build()
}

func (s *SysDictService) fillLocalizedTypeNames(items []*model.SysDictType, locale string) {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		ids = append(ids, item.ID)
	}
	if len(ids) == 0 {
		return
	}
	nameMap, err := model.NewSysDictTypeI18n().LocalizedNameMapByTypeIDs(ids, service.LocalePriority(locale))
	if err != nil {
		return
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		item.TypeName = nameMap[item.ID]
	}
}

func (s *SysDictService) fillLocalizedItemLabels(items []*model.SysDictItem, locale string) {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		ids = append(ids, item.ID)
	}
	if len(ids) == 0 {
		return
	}
	labelMap, err := model.NewSysDictItemI18n().LocalizedLabelMapByItemIDs(ids, service.LocalePriority(locale))
	if err != nil {
		return
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		item.Label = labelMap[item.ID]
	}
}

func (s *SysDictService) fillLocalizedItemLabelsFromSlice(items []model.SysDictItem, locale string) {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if len(ids) == 0 {
		return
	}
	labelMap, err := model.NewSysDictItemI18n().LocalizedLabelMapByItemIDs(ids, service.LocalePriority(locale))
	if err != nil {
		return
	}
	for i := range items {
		items[i].Label = labelMap[items[i].ID]
	}
}

func valueOrDefault(value *uint8, fallback uint8) uint8 {
	if value == nil {
		return fallback
	}
	return *value
}

func defaultStatus(current uint8, id uint) uint8 {
	if id == 0 && current == 0 {
		return 1
	}
	return current
}
