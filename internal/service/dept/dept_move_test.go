package dept

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

func init() {
	sql.Register("dept_move_sqlite", &sqlite3.SQLiteDriver{ConnectHook: func(c *sqlite3.SQLiteConn) error {
		if err := c.RegisterFunc("FIND_IN_SET", func(id int, pids string) int {
			for i, part := range strings.Split(pids, ",") {
				if part == strconv.Itoa(id) {
					return i + 1
				}
			}
			return 0
		}, true); err != nil {
			return err
		}
		return c.RegisterFunc("CONCAT", func(a, b string) string { return a + b }, true)
	}})
}

func newDeptMoveDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "dept_move_sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec("CREATE TABLE department (id integer primary key, pids text, level integer, deleted_at integer default 0, updated_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMoveDepartmentUpdatesEntireSubtree(t *testing.T) {
	cases := []struct {
		name, oldPids, newPids, oldChild, wantChild string
		oldLevel, newLevel                          uint8
	}{
		{"root_under_root", "0", "20", "10", "20,10", 1, 2},
		{"deeper", "1", "1,20", "1,10", "1,20,10", 2, 3},
		{"back_to_root", "1,20", "0", "1,20,10", "10", 3, 1},
		{"same_depth", "1", "20", "1,10", "20,10", 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newDeptMoveDB(t)
			for _, row := range []struct {
				id      int
				pids    string
				level   uint8
				deleted int
			}{
				{11, tc.oldChild, tc.oldLevel + 1, 0}, {12, tc.oldChild + ",11", tc.oldLevel + 2, 0},
				{21, "20", 2, 0}, {13, tc.oldChild, tc.oldLevel + 1, 1},
			} {
				if err := db.Exec("INSERT INTO department (id,pids,level,deleted_at) VALUES (?,?,?,?)", row.id, row.pids, row.level, row.deleted).Error; err != nil {
					t.Fatal(err)
				}
			}
			moved := model.NewDepartment()
			moved.ID = 10
			moved.Pids = tc.newPids
			moved.Level = tc.newLevel
			if err := db.Transaction(func(tx *gorm.DB) error { return updateDeptDescendants(tx, moved, tc.oldPids, tc.oldLevel) }); err != nil {
				t.Fatal(err)
			}
			for _, want := range []struct {
				id    int
				pids  string
				level uint8
			}{
				{11, tc.wantChild, tc.newLevel + 1}, {12, tc.wantChild + ",11", tc.newLevel + 2},
				{21, "20", 2}, {13, tc.oldChild, tc.oldLevel + 1},
			} {
				var got struct {
					Pids  string
					Level uint8
				}
				if err := db.Table("department").Where("id = ?", want.id).Take(&got).Error; err != nil {
					t.Fatal(err)
				}
				if got.Pids != want.pids || got.Level != want.level {
					t.Fatalf("id=%d got=%+v want=%+v", want.id, got, want)
				}
			}
			if tc.name == "root_under_root" && !utils2.WouldCauseCycle(20, 11, tc.wantChild) {
				t.Fatal("new ancestor must be rejected as a descendant's child")
			}
		})
	}
}

func TestMoveDepartmentRejectsExcessiveSubtreeDepth(t *testing.T) {
	db := newDeptMoveDB(t)
	if err := db.Exec("INSERT INTO department(id,pids,level) VALUES (11,'10',2),(12,'10,11',3)").Error; err != nil {
		t.Fatal(err)
	}
	moved := model.NewDepartment()
	moved.ID = 10
	moved.Pids = "1,2,3"
	moved.Level = 4
	err := db.Transaction(func(tx *gorm.DB) error { return updateDeptDescendants(tx, moved, "0", 1) })
	var business *e.BusinessError
	if !errors.As(err, &business) || business.GetCode() != e.MaxDeptDepth {
		t.Fatalf("expected max depth rejection, got %v", err)
	}
	var path string
	if err := db.Table("department").Where("id=12").Pluck("pids", &path).Error; err != nil {
		t.Fatal(err)
	}
	if path != "10,11" {
		t.Fatalf("rejected move changed child: %s", path)
	}
}
