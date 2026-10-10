package admin_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestDepartmentMoveKeepsSubtreeAndRejectsCycleOrExcessDepth(t *testing.T) {
	db, err := model.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	ids := []uint{}
	t.Cleanup(func() {
		if len(ids) > 0 {
			if err := db.Unscoped().Where("id IN ?", ids).Delete(&model.Department{}).Error; err != nil {
				t.Error(err)
			}
		}
	})
	seed := func(name string, parent *model.Department) *model.Department {
		t.Helper()
		row := model.NewDepartment()
		row.Name = "test-auto-move-" + name
		row.Code = fmt.Sprintf("test-auto-move-%d-%s", time.Now().UnixNano(), name)
		row.Pids = "0"
		row.Level = 1
		if parent != nil {
			row.Pid = parent.ID
			row.Level = parent.Level + 1
			row.Pids = fmt.Sprint(parent.ID)
			if parent.Pids != "0" {
				row.Pids = parent.Pids + "," + row.Pids
			}
		}
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, row.ID)
		return row
	}
	a := seed("a", nil)
	child := seed("child", a)
	grandchild := seed("grandchild", child)
	b := seed("b", nil)
	move := func(row *model.Department, parent uint, wantCode int) {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"id": row.ID, "pid": parent, "name": row.Name})
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		result := decodeResult(t, postRequest("/admin/v1/department/update", &body))
		if result.Code != wantCode {
			t.Fatalf("move %d to %d got code=%d want=%d", row.ID, parent, result.Code, wantCode)
		}
	}
	assertPath := func(id uint, pid uint, pids string, level uint8) {
		t.Helper()
		var got model.Department
		if err := db.First(&got, id).Error; err != nil {
			t.Fatal(err)
		}
		if got.Pid != pid || got.Pids != pids || got.Level != level {
			t.Fatalf("department %d pid=%d pids=%q level=%d want pid=%d pids=%q level=%d", id, got.Pid, got.Pids, got.Level, pid, pids, level)
		}
	}
	move(a, b.ID, e.SUCCESS)
	assertPath(child.ID, a.ID, fmt.Sprintf("%d,%d", b.ID, a.ID), 3)
	assertPath(grandchild.ID, child.ID, fmt.Sprintf("%d,%d,%d", b.ID, a.ID, child.ID), 4)
	move(b, grandchild.ID, e.ParentDeptInvalid)
	move(a, 0, e.SUCCESS)
	assertPath(child.ID, a.ID, fmt.Sprint(a.ID), 2)
	assertPath(grandchild.ID, child.ID, fmt.Sprintf("%d,%d", a.ID, child.ID), 3)
	parent2 := seed("parent2", b)
	parent3 := seed("parent3", parent2)
	move(a, parent3.ID, e.MaxDeptDepth)
	assertPath(a.ID, 0, "0", 1)
	assertPath(child.ID, a.ID, fmt.Sprint(a.ID), 2)
	assertPath(grandchild.ID, child.ID, fmt.Sprintf("%d,%d", a.ID, child.ID), 3)
}
