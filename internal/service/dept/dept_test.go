package dept

import (
	"testing"

	"gorm.io/gorm"
)

func TestDeptResolveDBReturnsProvidedTransaction(t *testing.T) {
	db := &gorm.DB{}

	service := NewDeptService()
	got, err := service.resolveDB(db)
	if err != nil {
		t.Fatalf("resolve db failed: %v", err)
	}
	if got != db {
		t.Fatalf("expected provided transaction db to be returned")
	}
}
