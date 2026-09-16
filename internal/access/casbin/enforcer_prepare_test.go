package casbinx

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestFailedEnforcerPreparationPreservesCurrentManager(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	before := casbinManager
	if _, err := PrepareEnforcerReload(db); err == nil {
		t.Fatal("expected missing policy table to reject candidate")
	}
	if casbinManager != before || casbinManager.errInit != before.errInit {
		t.Fatal("failed preparation mutated current manager")
	}
}
