package form

import "testing"

func TestSMSTestPayloadRequiresDraftConfig(t *testing.T) {
	if err := bindJSONBody(t, `{"country_code":"86","phone_number":"13800138000"}`, NewSMSTestPayload()); err == nil {
		t.Fatal("expected missing draft config to fail validation")
	}
}

func TestSMSTestPayloadAcceptsCompleteDraftConfig(t *testing.T) {
	body := `{
		"country_code":"86",
		"phone_number":"13800138000",
		"config":{
			"enabled":true,
			"providers":{"aliyun":{"enabled":true,"access_key_id":"id","access_key_secret":"********","endpoint":"https://dysmsapi.aliyuncs.com"}},
			"templates":{"aliyun":{"login_otp":{"sign_name":"Gin Layout","template_code":"SMS_LOGIN","variables":["code"]}}},
			"routes":{"login_otp":{"provider":"aliyun","template":"login_otp"}},
			"otp":{"ttl_seconds":300,"resend_interval_seconds":60,"max_attempts":5,"ip_limit_per_10_minutes":10,"phone_daily_limit":20,"global_daily_limit":1000}
		}
	}`
	payload := NewSMSTestPayload()
	if err := bindJSONBody(t, body, payload); err != nil {
		t.Fatalf("expected complete draft config to pass validation: %v", err)
	}
	if payload.Config == nil || !payload.Config.Enabled || payload.PhoneNumber != "13800138000" {
		t.Fatalf("unexpected bound payload: %#v", payload)
	}
}
