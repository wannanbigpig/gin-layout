package form

type SysDictTypeList struct {
	Paginate
	TypeCode string `form:"type_code" json:"type_code" binding:"omitempty,max=100"`
	TypeName string `form:"type_name" json:"type_name" binding:"omitempty,max=100"`
	Status   *uint8 `form:"status" json:"status" binding:"omitempty,oneof=0 1"`
}

type SysDictTypePayload struct {
	TypeCode     string            `form:"type_code" json:"type_code" label:"字典类型编码" binding:"required,max=100"`
	TypeNameI18n map[string]string `form:"type_name_i18n" json:"type_name_i18n" label:"字典类型名称多语言" binding:"required"`
	Status       *uint8            `form:"status" json:"status" label:"状态" binding:"omitempty,oneof=0 1"`
	Sort         uint              `form:"sort" json:"sort" label:"排序" binding:"omitempty"`
	Remark       string            `form:"remark" json:"remark" label:"备注" binding:"omitempty,max=255"`
}

type CreateSysDictType struct {
	SysDictTypePayload
}

type UpdateSysDictType struct {
	Id uint `form:"id" json:"id" label:"字典类型ID" binding:"required"`
	SysDictTypePayload
}

type SysDictItemList struct {
	Paginate
	TypeCode string `form:"type_code" json:"type_code" label:"字典类型编码" binding:"required,max=100"`
	Label    string `form:"label" json:"label" binding:"omitempty,max=100"`
	Value    string `form:"value" json:"value" binding:"omitempty,max=100"`
	Status   *uint8 `form:"status" json:"status" binding:"omitempty,oneof=0 1"`
}

type SysDictItemPayload struct {
	TypeCode  string            `form:"type_code" json:"type_code" label:"字典类型编码" binding:"required,max=100"`
	LabelI18n map[string]string `form:"label_i18n" json:"label_i18n" label:"字典标签多语言" binding:"required"`
	Value     string            `form:"value" json:"value" label:"字典值" binding:"required,max=100"`
	Color     string            `form:"color" json:"color" label:"展示颜色" binding:"omitempty,max=30"`
	TagType   string            `form:"tag_type" json:"tag_type" label:"标签类型" binding:"omitempty,max=30"`
	IsDefault *uint8            `form:"is_default" json:"is_default" label:"是否默认" binding:"omitempty,oneof=0 1"`
	Status    *uint8            `form:"status" json:"status" label:"状态" binding:"omitempty,oneof=0 1"`
	Sort      uint              `form:"sort" json:"sort" label:"排序" binding:"omitempty"`
	Remark    string            `form:"remark" json:"remark" label:"备注" binding:"omitempty,max=255"`
}

type CreateSysDictItem struct {
	SysDictItemPayload
}

type UpdateSysDictItem struct {
	Id uint `form:"id" json:"id" label:"字典项ID" binding:"required"`
	SysDictItemPayload
}

type SysDictOptionsQuery struct {
	TypeCode string `form:"type_code" json:"type_code" label:"字典类型编码" binding:"required,max=100"`
}

func NewSysDictTypeListQuery() *SysDictTypeList {
	return &SysDictTypeList{}
}

func NewCreateSysDictTypeForm() *CreateSysDictType {
	return &CreateSysDictType{}
}

func NewUpdateSysDictTypeForm() *UpdateSysDictType {
	return &UpdateSysDictType{}
}

func NewSysDictItemListQuery() *SysDictItemList {
	return &SysDictItemList{}
}

func NewCreateSysDictItemForm() *CreateSysDictItem {
	return &CreateSysDictItem{}
}

func NewUpdateSysDictItemForm() *UpdateSysDictItem {
	return &UpdateSysDictItem{}
}

func NewSysDictOptionsQuery() *SysDictOptionsQuery {
	return &SysDictOptionsQuery{}
}
