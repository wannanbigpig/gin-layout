package access

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/redisutil"
)

func TestDataScopeConstants(t *testing.T) {
	if DataScopeAll != 1 {
		t.Fatalf("DataScopeAll = %d, want 1", DataScopeAll)
	}
	if DataScopeDeptAndChildren != 2 {
		t.Fatalf("DataScopeDeptAndChildren = %d, want 2", DataScopeDeptAndChildren)
	}
	if DataScopeDept != 3 {
		t.Fatalf("DataScopeDept = %d, want 3", DataScopeDept)
	}
	if DataScopeSelf != 4 {
		t.Fatalf("DataScopeSelf = %d, want 4", DataScopeSelf)
	}
	if DataScopeCustom != 5 {
		t.Fatalf("DataScopeCustom = %d, want 5", DataScopeCustom)
	}
}

func TestDataScopeFilterBuild_SuperAdminShortCircuit(t *testing.T) {
	f := NewDataScopeFilter()
	input := DataScopeInput{UID: 1, IsSuperAdmin: true}
	cond, args := f.Build(context.Background(), input, "dept_id", "created_by")
	if cond != "" {
		t.Fatalf("super admin should get empty condition, got: %s", cond)
	}
	if len(args) != 0 {
		t.Fatalf("super admin should get nil args, got: %v", args)
	}
}

func TestDataScopeFilterBuildFromSnapshot_ScopeAll(t *testing.T) {
	f := NewDataScopeFilter()
	snapshot := DataScopeSnapshot{ScopeAll: true, DeptCol: "dept_id", OwnerCol: "created_by"}
	cond, args := f.BuildFromSnapshot(snapshot)
	if cond != "" {
		t.Fatalf("scope all should get empty condition, got: %s", cond)
	}
	if len(args) != 0 {
		t.Fatalf("scope all should get nil args, got: %v", args)
	}
}

func TestDataScopeFilterBuildFromSnapshot_OnlyOwner(t *testing.T) {
	f := NewDataScopeFilter()
	snapshot := DataScopeSnapshot{
		ScopeAll: false,
		OwnerUID: 42,
		DeptCol:  "dept_id",
		OwnerCol: "created_by",
	}
	cond, args := f.BuildFromSnapshot(snapshot)
	expected := "created_by = ?"
	if cond != expected {
		t.Fatalf("expected %q, got %q", expected, cond)
	}
	if len(args) != 1 || args[0] != uint(42) {
		t.Fatalf("expected args [42], got %v", args)
	}
}

func TestDataScopeFilterBuildFromSnapshot_OnlyDepts(t *testing.T) {
	f := NewDataScopeFilter()
	snapshot := DataScopeSnapshot{
		ScopeAll: false,
		DeptIDs:  []uint{10, 20},
		DeptCol:  "dept_id",
		OwnerCol: "created_by",
	}
	cond, args := f.BuildFromSnapshot(snapshot)
	expected := "dept_id IN (?)"
	if cond != expected {
		t.Fatalf("expected %q, got %q", expected, cond)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	deptIDs, ok := args[0].([]uint)
	if !ok {
		t.Fatalf("expected []uint arg, got %T", args[0])
	}
	if len(deptIDs) != 2 {
		t.Fatalf("expected 2 dept IDs, got %d", len(deptIDs))
	}
}

func TestDataScopeFilterBuildFromSnapshot_DeptsAndOwner(t *testing.T) {
	f := NewDataScopeFilter()
	snapshot := DataScopeSnapshot{
		ScopeAll: false,
		DeptIDs:  []uint{10, 20},
		OwnerUID: 42,
		DeptCol:  "t.dept_id",
		OwnerCol: "t.created_by",
	}
	cond, args := f.BuildFromSnapshot(snapshot)
	expected := "(t.dept_id IN (?) OR t.created_by = ?)"
	if cond != expected {
		t.Fatalf("expected %q, got %q", expected, cond)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
}

func TestDataScopeFilterBuildFromSnapshot_EmptyDenyAll(t *testing.T) {
	f := NewDataScopeFilter()
	snapshot := DataScopeSnapshot{
		ScopeAll: false,
		DeptIDs:  nil,
		OwnerUID: 0,
		DeptCol:  "dept_id",
		OwnerCol: "created_by",
	}
	cond, _ := f.BuildFromSnapshot(snapshot)
	if cond != "1=0" {
		t.Fatalf("empty scope should return 1=0, got: %s", cond)
	}
}

func TestDataScopeFilterBuild_NonSuperAdminNoCache(t *testing.T) {
	f := NewDataScopeFilter()
	input := DataScopeInput{UID: 999, IsSuperAdmin: false}
	// Build will try cache then DB; without DB it should deny access (1=0)
	cond, _ := f.Build(context.Background(), input, "dept_id", "created_by")
	// Without DB/Redis, computeScope will fail → 1=0
	if cond != "1=0" {
		t.Fatalf("expected 1=0 when DB unavailable, got: %q", cond)
	}
}

func TestDataScopeFilterBuildSingleflightSharedAcrossInstances(t *testing.T) {
	useDataScopeTestVersion(t, 1)
	previousCompute := computeScopeForSingleflight
	defer func() {
		computeScopeForSingleflight = previousCompute
	}()

	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	computeScopeForSingleflight = func(_ *DataScopeFilter, _ context.Context, uid uint) (*scopeResult, error) {
		if uid != 42 {
			t.Fatalf("unexpected uid: %d", uid)
		}
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return &scopeResult{
			OwnerUID:    uid,
			HasOwnerUID: true,
		}, nil
	}

	input := DataScopeInput{UID: 42, IsSuperAdmin: false}
	results := make(chan string, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cond, _ := NewDataScopeFilter().Build(context.Background(), input, "dept_id", "created_by")
		results <- cond
	}()
	<-started
	go func() {
		defer wg.Done()
		cond, _ := NewDataScopeFilter().Build(context.Background(), input, "dept_id", "created_by")
		results <- cond
	}()
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one shared compute call, got %d", got)
	}
	for cond := range results {
		if cond != "created_by = ?" {
			t.Fatalf("unexpected condition: %s", cond)
		}
	}
}

func TestDataScopeFilterResolveDepartmentScopeUsesMergedScope(t *testing.T) {
	useDataScopeTestVersion(t, 1)
	previousCompute := computeScopeForSingleflight
	defer func() {
		computeScopeForSingleflight = previousCompute
	}()

	uid := uint(900004)
	computeScopeForSingleflight = func(_ *DataScopeFilter, _ context.Context, gotUID uint) (*scopeResult, error) {
		if gotUID != uid {
			t.Fatalf("unexpected uid: %d", gotUID)
		}
		return &scopeResult{
			DeptIDs:    []uint{10, 20},
			HasDeptIDs: true,
			OwnerUID:   uid,
		}, nil
	}

	scopeAll, deptIDs, err := NewDataScopeFilter().ResolveDepartmentScope(context.Background(), DataScopeInput{UID: uid})
	if err != nil {
		t.Fatalf("resolve department scope failed: %v", err)
	}
	if scopeAll {
		t.Fatal("expected restricted department scope")
	}
	if len(deptIDs) != 2 || deptIDs[0] != 10 || deptIDs[1] != 20 {
		t.Fatalf("unexpected department scope: %v", deptIDs)
	}
}

func TestDataScopeFilterCacheKey(t *testing.T) {
	f := NewDataScopeFilter()
	k1 := f.cacheKey(1)
	k2 := f.cacheKey(2)
	if k1 == k2 {
		t.Fatalf("different UIDs should produce different cache keys")
	}
	expected := redisutil.Channel("data_scope:1")
	if k1 != expected {
		t.Fatalf("expected %q, got %q", expected, k1)
	}
}

func TestDataScopeCacheInvalidationMakesCapturedVersionStale(t *testing.T) {
	testDataScopeMutationDuringCompute(t, false)
}

func TestDataScopeGlobalInvalidationMakesAllCapturedVersionsStale(t *testing.T) {
	testDataScopeMutationDuringCompute(t, true)
}

func TestDataScopeInputFields(t *testing.T) {
	input := DataScopeInput{UID: 100, IsSuperAdmin: true}
	if input.UID != 100 {
		t.Fatalf("UID = %d, want 100", input.UID)
	}
	if !input.IsSuperAdmin {
		t.Fatal("IsSuperAdmin should be true")
	}
}

func TestDataScopeSnapshotJSON(t *testing.T) {
	snapshot := DataScopeSnapshot{
		ScopeAll: false,
		DeptIDs:  []uint{1, 2, 3},
		OwnerUID: 42,
		DeptCol:  "dept_id",
		OwnerCol: "created_by",
	}
	// Verify fields are accessible
	if snapshot.ScopeAll {
		t.Fatal("ScopeAll should be false")
	}
	if len(snapshot.DeptIDs) != 3 {
		t.Fatalf("DeptIDs length = %d, want 3", len(snapshot.DeptIDs))
	}
	if snapshot.OwnerUID != 42 {
		t.Fatalf("OwnerUID = %d, want 42", snapshot.OwnerUID)
	}
}

func TestBuildFromSnapshot_TablePrefixColumns(t *testing.T) {
	f := NewDataScopeFilter()
	snapshot := DataScopeSnapshot{
		ScopeAll: false,
		DeptIDs:  []uint{5},
		OwnerCol: "created_by",
		DeptCol:  "p.dept_id",
	}
	cond, _ := f.BuildFromSnapshot(snapshot)
	if cond != "p.dept_id IN (?)" {
		t.Fatalf("expected table-prefixed column, got: %s", cond)
	}
}

func TestCollectDeptAndChildrenFromDBUsesParentTree(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := createDepartmentSQLiteTable(db); err != nil {
		t.Fatalf("create department table failed: %v", err)
	}
	rows := []model.Department{
		{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 1}}, Name: "root-a", Pid: 0},
		{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 2}}, Name: "child-a", Pid: 1},
		{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 3}}, Name: "grand-a", Pid: 2},
		{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 4}}, Name: "root-b", Pid: 0},
		{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 5}}, Name: "child-b", Pid: 4},
		{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 6}}, Name: "other", Pid: 0},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed departments failed: %v", err)
	}

	got, err := collectDeptAndChildrenFromDB(db, []uint{1, 4})
	if err != nil {
		t.Fatalf("collect dept children failed: %v", err)
	}
	want := map[uint]struct{}{1: {}, 2: {}, 3: {}, 4: {}, 5: {}}
	if len(got) != len(want) {
		t.Fatalf("unexpected dept ids: got=%v want=%v", got, want)
	}
	for _, id := range got {
		if _, ok := want[id]; !ok {
			t.Fatalf("unexpected dept id %d in %v", id, got)
		}
	}
}

func createDepartmentSQLiteTable(db *gorm.DB) error {
	return db.Exec(`
CREATE TABLE department (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at datetime,
	updated_at datetime,
	deleted_at INTEGER NOT NULL DEFAULT 0,
	code TEXT NOT NULL DEFAULT '',
	is_system INTEGER NOT NULL DEFAULT 0,
	pid INTEGER NOT NULL DEFAULT 0,
	pids TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	level INTEGER NOT NULL DEFAULT 1,
	sort INTEGER NOT NULL DEFAULT 0,
	children_num INTEGER NOT NULL DEFAULT 0,
	user_number INTEGER NOT NULL DEFAULT 0
)`).Error
}
