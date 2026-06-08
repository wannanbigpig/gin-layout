package form

import "testing"

func TestRoleStatusUsesBinaryEnum(t *testing.T) {
	if err := bindJSONBody(t, `{"name":"审计员","status":0}`, NewCreateRoleForm()); err != nil {
		t.Fatalf("expected status=0 to pass validation, got %v", err)
	}
	if err := bindJSONBody(t, `{"name":"审计员","status":1}`, NewCreateRoleForm()); err != nil {
		t.Fatalf("expected status=1 to pass validation, got %v", err)
	}
	if err := bindJSONBody(t, `{"name":"审计员","status":2}`, NewCreateRoleForm()); err == nil {
		t.Fatal("expected status=2 to fail validation")
	}
}

func TestUpdateRoleRejectsCode(t *testing.T) {
	body := `{"id":1,"code":"auditor_new","name":"审计员","status":1}`
	if err := bindJSONBody(t, body, NewUpdateRoleForm()); err == nil {
		t.Fatal("expected update role code to fail validation")
	}
}

func TestUpdateRoleAllowsOmittedCode(t *testing.T) {
	body := `{"id":1,"name":"审计员","status":1}`
	if err := bindJSONBody(t, body, NewUpdateRoleForm()); err != nil {
		t.Fatalf("expected update role without code to pass validation, got %v", err)
	}
}
