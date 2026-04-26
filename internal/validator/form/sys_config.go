package form

type SysConfigList struct {
	Paginate
	ConfigKey  string `form:"config_key" json:"config_key" binding:"omitempty,max=100"`
	ConfigName string `form:"config_name" json:"config_name" binding:"omitempty,max=100"`
	GroupCode  string `form:"group_code" json:"group_code" binding:"omitempty,max=60"`
	ValueType  string `form:"value_type" json:"value_type" binding:"omitempty,oneof=string number bool json"`
	Status     *uint8 `form:"status" json:"status" binding:"omitempty,oneof=0 1"`
}

type SysConfigPayload struct {
	ConfigKey      string            `form:"config_key" json:"config_key" label:"参数键名" binding:"required,max=100"`
	ConfigNameI18n map[string]string `form:"config_name_i18n" json:"config_name_i18n" label:"参数名称多语言" binding:"required"`
	ConfigValue    string            `form:"config_value" json:"config_value" label:"参数值" binding:"omitempty"`
	ValueType      string            `form:"value_type" json:"value_type" label:"值类型" binding:"required,oneof=string number bool json"`
	GroupCode      string            `form:"group_code" json:"group_code" label:"参数分组" binding:"omitempty,max=60"`
	IsSensitive    *uint8            `form:"is_sensitive" json:"is_sensitive" label:"是否敏感" binding:"omitempty,oneof=0 1"`
	Status         *uint8            `form:"status" json:"status" label:"状态" binding:"omitempty,oneof=0 1"`
	Sort           uint              `form:"sort" json:"sort" label:"排序" binding:"omitempty"`
	Remark         string            `form:"remark" json:"remark" label:"备注" binding:"omitempty,max=255"`
}

type CreateSysConfig struct {
	SysConfigPayload
}

type UpdateSysConfig struct {
	Id uint `form:"id" json:"id" label:"参数ID" binding:"required"`
	SysConfigPayload
}

type SysConfigKeyQuery struct {
	ConfigKey string `form:"config_key" json:"config_key" label:"参数键名" binding:"required,max=100"`
}

func NewSysConfigListQuery() *SysConfigList {
	return &SysConfigList{}
}

func NewCreateSysConfigForm() *CreateSysConfig {
	return &CreateSysConfig{}
}

func NewUpdateSysConfigForm() *UpdateSysConfig {
	return &UpdateSysConfig{}
}

func NewSysConfigKeyQuery() *SysConfigKeyQuery {
	return &SysConfigKeyQuery{}
}
