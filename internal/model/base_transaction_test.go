package model

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type transactionListModel struct {
	BaseModel
	Name string
}

func (*transactionListModel) TableName() string { return "transaction_list_items" }

func TestGenericQueriesUseBoundTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec("CREATE TABLE transaction_list_items (id INTEGER PRIMARY KEY, created_at datetime, updated_at datetime, name text)").Error; err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("rollback test")
	err = db.Transaction(func(tx *gorm.DB) error {
		item := BindModel(&transactionListModel{Name: "uncommitted"})
		item.SetDB(tx)
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		list, err := ListE(item, "name = ?", []any{"uncommitted"})
		if err != nil || len(list) != 1 {
			t.Fatalf("ListE escaped transaction: %v, %v", list, err)
		}
		total, page, err := ListPageE(item, 1, 10, "", nil)
		if err != nil || total != 1 || len(page) != 1 {
			t.Fatalf("ListPageE escaped transaction: total=%d rows=%d error=%v", total, len(page), err)
		}
		ids, err := VerifyExistingIDs(item, []uint{item.ID})
		if err != nil || len(ids) != 1 {
			t.Fatalf("VerifyExistingIDs escaped transaction: %v, %v", ids, err)
		}
		names, err := ExtractColumnsByCondition[transactionListModel, *transactionListModel, string](item, "name", "id = ?", item.ID)
		if err != nil || len(names) != 1 || names[0] != "uncommitted" {
			t.Fatalf("ExtractColumns escaped transaction: %v, %v", names, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&transactionListModel{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rollback failed: count=%d error=%v", count, err)
	}
}
