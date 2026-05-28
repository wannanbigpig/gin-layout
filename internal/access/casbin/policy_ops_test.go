package casbinx

import (
	"fmt"
	"testing"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExecuteReusesCachedEnforcerWithinSameTransaction(t *testing.T) {
	db := newCasbinPolicyTestDB(t)
	var calls int
	restore := swapNewEnforcerFromDBForTesting(func(_ casbinmodel.Model, _ *gorm.DB) (*casbin.Enforcer, error) {
		calls++
		return &casbin.Enforcer{}, nil
	})
	defer restore()

	enforcer := &CasbinEnforcer{}
	var pointers []string
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := enforcer.execute(tx, func(runtime casbin.IEnforcer) error {
			pointers = append(pointers, fmt.Sprintf("%p", runtime))
			return nil
		}); err != nil {
			return err
		}
		return enforcer.execute(tx, func(runtime casbin.IEnforcer) error {
			pointers = append(pointers, fmt.Sprintf("%p", runtime))
			return nil
		})
	}); err != nil {
		t.Fatalf("transaction execute failed: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected one tx enforcer build, got %d", calls)
	}
	if len(pointers) != 2 || pointers[0] != pointers[1] {
		t.Fatalf("expected same cached enforcer instance, got %#v", pointers)
	}
}

func TestExecuteBuildsNewEnforcerForDifferentTransactions(t *testing.T) {
	db := newCasbinPolicyTestDB(t)
	calls := 0
	restore := swapNewEnforcerFromDBForTesting(func(_ casbinmodel.Model, _ *gorm.DB) (*casbin.Enforcer, error) {
		calls++
		return &casbin.Enforcer{}, nil
	})
	defer restore()

	enforcer := &CasbinEnforcer{}
	for i := 0; i < 2; i++ {
		if err := db.Transaction(func(tx *gorm.DB) error {
			return enforcer.execute(tx, func(runtime casbin.IEnforcer) error { return nil })
		}); err != nil {
			t.Fatalf("transaction %d execute failed: %v", i, err)
		}
	}

	if calls != 2 {
		t.Fatalf("expected one tx enforcer per transaction, got %d", calls)
	}
}

func TestExecuteRejectsNonTransactionDB(t *testing.T) {
	db := newCasbinPolicyTestDB(t)
	enforcer := &CasbinEnforcer{}
	if err := enforcer.execute(db, func(runtime casbin.IEnforcer) error { return nil }); err == nil {
		t.Fatal("expected execute to reject non-transaction db")
	}
}

func newCasbinPolicyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	return db
}

func swapNewEnforcerFromDBForTesting(fn func(casbinmodel.Model, *gorm.DB) (*casbin.Enforcer, error)) func() {
	previous := newEnforcerFromDBFunc
	newEnforcerFromDBFunc = fn
	return func() {
		newEnforcerFromDBFunc = previous
	}
}
