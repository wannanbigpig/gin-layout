package casbinx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakePolicyVersionAccess struct {
	currentVersion uint64
	currentErr     error
	snapshot       *casbin.Enforcer
	loadedVersion  uint64
	loadErr        error
	loads          atomic.Int32
}

func (f *fakePolicyVersionAccess) Current(context.Context) (uint64, error) {
	return f.currentVersion, f.currentErr
}

func (f *fakePolicyVersionAccess) LoadSnapshot(context.Context, casbinmodel.Model) (*casbin.Enforcer, uint64, error) {
	f.loads.Add(1)
	if f.loadErr != nil {
		return nil, 0, f.loadErr
	}
	return f.snapshot, f.loadedVersion, nil
}

func (*fakePolicyVersionAccess) RecordLoaded(context.Context, uint64) error { return nil }

func TestEnforceReloadsOutdatedPolicyOnceForConcurrentRequests(t *testing.T) {
	oldSnapshot := newVersionTestEnforcer(t, false)
	newSnapshot := newVersionTestEnforcer(t, true)
	versions := &fakePolicyVersionAccess{currentVersion: 2, snapshot: newSnapshot, loadedVersion: 2}
	managed := &CasbinEnforcer{
		Enforcer:     oldSnapshot,
		policyState:  &sharedPolicyState{loadedVersion: 1},
		versionStore: versions,
	}

	const concurrency = 24
	start := make(chan struct{})
	errs := make(chan error, concurrency)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			allowed, err := managed.Enforce("user", "/resource", "GET")
			if err != nil {
				errs <- err
				return
			}
			if !allowed {
				errs <- errors.New("expected refreshed policy to allow request")
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := versions.loads.Load(); got != 1 {
		t.Fatalf("expected one policy reload, got %d", got)
	}
}

func TestEnforceFailsClosedWhenVersionCannotBeRead(t *testing.T) {
	managed := &CasbinEnforcer{
		Enforcer:    newVersionTestEnforcer(t, true),
		policyState: &sharedPolicyState{loadedVersion: 1},
		versionStore: &fakePolicyVersionAccess{
			currentErr: errors.New("version unavailable"),
		},
	}
	allowed, err := managed.Enforce("user", "/resource", "GET")
	if err == nil || allowed {
		t.Fatalf("expected fail-closed version check, allowed=%v err=%v", allowed, err)
	}
}

func TestReplacePermissionsAdvancesVersionInSameTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE permission_policy_versions (id integer primary key, version integer not null, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO permission_policy_versions (id, version) VALUES (1, 1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE casbin_rule (id integer primary key autoincrement, ptype text, v0 text, v1 text, v2 text, v3 text, v4 text, v5 text)`).Error; err != nil {
		t.Fatal(err)
	}

	previous := defaultPolicyVersionStore
	defaultPolicyVersionStore = &policyVersionStore{
		dbProvider:    func() *gorm.DB { return db },
		redisProvider: func() *redis.Client { return nil },
		cacheTTL:      policyVersionCacheTTL,
		dirtyTTL:      policyVersionDirtyTTL,
	}
	t.Cleanup(func() { defaultPolicyVersionStore = previous })

	if err := db.Transaction(func(tx *gorm.DB) error {
		return replacePermissionsInDB(tx, map[string][][]string{"user": {{"/one", "GET"}}})
	}); err != nil {
		t.Fatal(err)
	}
	assertStoredPolicyVersion(t, db, 2, "/one")

	wantRollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := replacePermissionsInDB(tx, map[string][][]string{"user": {{"/two", "POST"}}}); err != nil {
			return err
		}
		return wantRollback
	})
	if !errors.Is(err, wantRollback) {
		t.Fatalf("expected rollback marker, got %v", err)
	}
	assertStoredPolicyVersion(t, db, 2, "/one")
}

func TestPolicyVersionRedisFailureUsesCooldown(t *testing.T) {
	store := &policyVersionStore{}
	store.markRedisFailure()
	if store.shouldTryRedis() {
		t.Fatal("expected Redis attempts to pause during cooldown")
	}
	store.redisRetryAt.Store(time.Now().Add(-time.Millisecond).UnixNano())
	if !store.shouldTryRedis() {
		t.Fatal("expected Redis attempts to resume after cooldown")
	}
}

func TestSharedDataScopeVersionAdvancesWithoutPolicyRowsAndRollsBack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE permission_policy_versions (id integer primary key, version integer not null, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO permission_policy_versions (id, version) VALUES (1, 7)`).Error; err != nil {
		t.Fatal(err)
	}
	previous := defaultPolicyVersionStore
	defaultPolicyVersionStore = &policyVersionStore{
		dbProvider:    func() *gorm.DB { return db },
		redisProvider: func() *redis.Client { return nil },
		cacheTTL:      policyVersionCacheTTL,
		dirtyTTL:      policyVersionDirtyTTL,
	}
	t.Cleanup(func() { defaultPolicyVersionStore = previous })
	assertVersion := func(want uint64) {
		t.Helper()
		got, err := CurrentPermissionVersion(context.Background())
		if err != nil || got != want {
			t.Fatalf("DB fallback version=%d want=%d err=%v", got, want, err)
		}
	}
	assertVersion(7)
	if err := db.Transaction(AdvancePermissionVersion); err != nil {
		t.Fatal(err)
	}
	assertVersion(8)
	rollback := errors.New("tree change failed")
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := AdvancePermissionVersion(tx); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("expected rollback, got %v", err)
	}
	assertVersion(8)
	if err := db.Exec(`DROP TABLE permission_policy_versions`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := CurrentPermissionVersion(context.Background()); err == nil {
		t.Fatal("missing authority must not return a usable version")
	}
}

func TestPolicyVersionCurrentCoalescesConcurrentDBFallback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:policy_version_singleflight?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE permission_policy_versions (id integer primary key, version integer not null, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO permission_policy_versions (id, version) VALUES (1, 9)`).Error; err != nil {
		t.Fatal(err)
	}

	var queries atomic.Int32
	queryEntered := make(chan struct{})
	releaseQuery := make(chan struct{})
	if err := db.Callback().Query().Before("gorm:query").Register("test:count_policy_version_query", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "permission_policy_versions" {
			if queries.Add(1) == 1 {
				close(queryEntered)
				<-releaseQuery
			}
		}
	}); err != nil {
		t.Fatal(err)
	}

	store := &policyVersionStore{
		dbProvider:    func() *gorm.DB { return db },
		redisProvider: func() *redis.Client { return nil },
		cacheTTL:      policyVersionCacheTTL,
		dirtyTTL:      policyVersionDirtyTTL,
	}
	const concurrency = 32
	start := make(chan struct{})
	results := make(chan uint64, concurrency)
	errs := make(chan error, concurrency)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			version, err := store.Current(context.Background())
			if err != nil {
				errs <- err
				return
			}
			results <- version
		}()
	}
	close(start)
	select {
	case <-queryEntered:
	case <-time.After(time.Second):
		t.Fatal("DB fallback query did not start")
	}
	time.Sleep(20 * time.Millisecond)
	close(releaseQuery)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	for version := range results {
		if version != 9 {
			t.Errorf("unexpected version %d", version)
		}
	}
	if got := queries.Load(); got != 1 {
		t.Fatalf("expected one coalesced DB query, got %d", got)
	}
}

func newVersionTestEnforcer(t *testing.T, allowed bool) *casbin.Enforcer {
	t.Helper()
	m, err := casbinmodel.NewModelFromString("[request_definition]\nr = sub, obj, act\n[policy_definition]\np = sub, obj, act\n[policy_effect]\ne = some(where (p.eft == allow))\n[matchers]\nm = r.sub == p.sub && r.obj == p.obj && r.act == p.act")
	if err != nil {
		t.Fatal(err)
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		if ok, err := enforcer.AddPolicy("user", "/resource", "GET"); err != nil || !ok {
			t.Fatalf("add policy: ok=%v err=%v", ok, err)
		}
	}
	return enforcer
}

func assertStoredPolicyVersion(t *testing.T, db *gorm.DB, version uint64, route string) {
	t.Helper()
	var row permissionPolicyVersion
	if err := db.First(&row, policyVersionRowID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Version != version {
		t.Fatalf("unexpected version: got %d want %d", row.Version, version)
	}
	var count int64
	if err := db.Model(&casbinPolicyRule{}).Where("v0 = ? AND v1 = ?", "user", route).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected persisted route %s, got count %d", route, count)
	}
}
