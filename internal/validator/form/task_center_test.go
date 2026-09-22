package form

import "testing"

func TestTaskRunListRejectsInvalidStatus(t *testing.T) {
	err := bindJSONBody(t, `{"status":"succeeded"}`, NewTaskRunListQuery())
	if err == nil {
		t.Fatal("expected invalid task run status to fail validation")
	}
}

func TestTaskRunListAllowsKnownStatus(t *testing.T) {
	err := bindJSONBody(t, `{"status":"success"}`, NewTaskRunListQuery())
	if err != nil {
		t.Fatalf("expected known task run status to pass validation, got %v", err)
	}
}

func TestTaskRunListValidatesTimeBasis(t *testing.T) {
	if err := bindJSONBody(t, `{"time_basis":"finished_at"}`, NewTaskRunListQuery()); err != nil {
		t.Fatalf("expected finished_at time basis to pass validation, got %v", err)
	}
	if err := bindJSONBody(t, `{"time_basis":"updated_at"}`, NewTaskRunListQuery()); err == nil {
		t.Fatal("expected unsupported time basis to fail validation")
	}
}

func TestCronTaskStateListRejectsInvalidLastStatus(t *testing.T) {
	err := bindJSONBody(t, `{"last_status":"succeeded"}`, NewCronTaskStateListQuery())
	if err == nil {
		t.Fatal("expected invalid cron task status to fail validation")
	}
}

func TestCronTaskStateListAllowsKnownLastStatus(t *testing.T) {
	err := bindJSONBody(t, `{"last_status":"retrying"}`, NewCronTaskStateListQuery())
	if err != nil {
		t.Fatalf("expected known cron task status to pass validation, got %v", err)
	}
}
