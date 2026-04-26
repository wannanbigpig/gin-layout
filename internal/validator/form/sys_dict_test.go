package form

import "testing"

func TestCreateSysDictTypeRejectsInvalidStatus(t *testing.T) {
	err := bindJSONBody(t, `{"type_code":"test_type","type_name_i18n":{"zh-CN":"测试字典"},"status":2}`, NewCreateSysDictTypeForm())
	if err == nil {
		t.Fatal("expected status=2 to fail validation")
	}
}

func TestCreateSysDictItemAllowsValidBinaryFlags(t *testing.T) {
	body := `{"type_code":"test_type","label_i18n":{"zh-CN":"启用","en-US":"Enabled"},"value":"1","is_default":1,"status":0}`
	if err := bindJSONBody(t, body, NewCreateSysDictItemForm()); err != nil {
		t.Fatalf("expected sys_dict_item payload to pass validation, got %v", err)
	}
}

func TestCreateSysDictItemRejectsInvalidDefaultFlag(t *testing.T) {
	body := `{"type_code":"test_type","label_i18n":{"zh-CN":"启用"},"value":"1","is_default":2}`
	err := bindJSONBody(t, body, NewCreateSysDictItemForm())
	if err == nil {
		t.Fatal("expected is_default=2 to fail validation")
	}
}
