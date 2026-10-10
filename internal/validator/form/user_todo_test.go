package form

import "testing"

func TestTodoReminderLeadTimePreservesLegacyRange(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"in_app_enabled":true,"sms_enabled":false,"lead_minutes":0}`, true},
		{`{"in_app_enabled":true,"sms_enabled":false,"lead_minutes":43200}`, true},
		{`{"in_app_enabled":true,"sms_enabled":false,"lead_minutes":-1}`, false},
		{`{"in_app_enabled":true,"sms_enabled":false,"lead_minutes":43201}`, false},
	} {
		if err := bindJSONBody(t, tc.body, NewUserTodoReminderPreferenceForm()); (err == nil) != tc.valid {
			t.Fatalf("body=%s error=%v", tc.body, err)
		}
	}
}
