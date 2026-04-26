package form

import "testing"

func TestCreateSysConfigAllowsValidValueTypes(t *testing.T) {
	cases := []string{
		`{"config_key":"feature.demo","config_name_i18n":{"zh-CN":"演示开关","en-US":"Feature Toggle"},"config_value":"true","value_type":"bool","status":1}`,
		`{"config_key":"number.demo","config_name_i18n":{"zh-CN":"数字参数"},"config_value":"10.5","value_type":"number","status":0}`,
		`{"config_key":"json.demo","config_name_i18n":{"zh-CN":"JSON参数"},"config_value":"{\"a\":1}","value_type":"json","status":1}`,
	}
	for _, body := range cases {
		if err := bindJSONBody(t, body, NewCreateSysConfigForm()); err != nil {
			t.Fatalf("expected sys_config payload to pass validation, got %v", err)
		}
	}
}

func TestCreateSysConfigRejectsInvalidValueType(t *testing.T) {
	err := bindJSONBody(t, `{"config_key":"feature.demo","config_name_i18n":{"zh-CN":"演示开关"},"config_value":"1","value_type":"yaml"}`, NewCreateSysConfigForm())
	if err == nil {
		t.Fatal("expected unsupported value_type to fail validation")
	}
}

func TestSysConfigListRejectsInvalidStatus(t *testing.T) {
	err := bindJSONBody(t, `{"status":2}`, NewSysConfigListQuery())
	if err == nil {
		t.Fatal("expected status=2 to fail validation")
	}
}
