package form

import (
	"encoding/json"
	"testing"
)

func TestUpdateAdminUserDeptIDsDistinguishEmptyArrayFromAbsentField(t *testing.T) {
	var withEmpty UpdateAdminUser
	if err := json.Unmarshal([]byte(`{"id":1,"dept_ids":[]}`), &withEmpty); err != nil {
		t.Fatalf("unmarshal with empty dept_ids: %v", err)
	}
	if withEmpty.DeptIds == nil {
		t.Fatal("expected dept_ids empty array to keep non-nil pointer")
	}
	if len(*withEmpty.DeptIds) != 0 {
		t.Fatalf("expected empty dept_ids, got %#v", *withEmpty.DeptIds)
	}

	var withoutField UpdateAdminUser
	if err := json.Unmarshal([]byte(`{"id":1}`), &withoutField); err != nil {
		t.Fatalf("unmarshal without dept_ids: %v", err)
	}
	if withoutField.DeptIds != nil {
		t.Fatalf("expected absent dept_ids to stay nil, got %#v", *withoutField.DeptIds)
	}
}

func TestCreateAdminUserRequiresPassword(t *testing.T) {
	err := bindJSONBody(t, `{"username":"admin_user","nickname":"管理员"}`, NewCreateAdminUser())
	if err == nil {
		t.Fatal("expected missing password to fail validation")
	}
}

func TestCreateAdminUserAllowsRequiredFields(t *testing.T) {
	err := bindJSONBody(t, `{"username":"admin_user","nickname":"管理员","password":"123456"}`, NewCreateAdminUser())
	if err != nil {
		t.Fatalf("expected required create fields to pass validation, got %v", err)
	}
}

func TestUpdateProfileRequiresCurrentPasswordOnlyWhenChangingPassword(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "ordinary profile update", body: `{"nickname":"新昵称"}`},
		{name: "password without current password", body: `{"password":"new-password"}`, wantErr: true},
		{name: "password with current password", body: `{"current_password":"old-password","password":"new-password"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bindJSONBody(t, tt.body, NewUpdateProfile())
			if (err != nil) != tt.wantErr {
				t.Fatalf("validation error = %v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateProfileEmailCanBeCleared(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
		wantNil    bool
	}{
		{"omitted", `{}`, false, true},
		{"clear", `{"email":""}`, false, false},
		{"valid", `{"email":"user@example.com"}`, false, false},
		{"invalid", `{"email":"invalid-email"}`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := NewUpdateProfile()
			err := bindJSONBody(t, tc.body, payload)
			if (err != nil) != tc.wantErr {
				t.Fatalf("bind error=%v wantErr=%v", err, tc.wantErr)
			}
			if (payload.Email == nil) != tc.wantNil {
				t.Fatal("email omission must remain distinct from clearing")
			}
		})
	}
}
