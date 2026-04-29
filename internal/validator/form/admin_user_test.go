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
