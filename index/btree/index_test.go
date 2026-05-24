package btree_test

import (
	"path/filepath"
	"testing"

	btree "godb/index/btree"
	"godb/record"
	"godb/server"
)

func TestBTreeIndex_InsertAndSearch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	db, err := server.NewGoDB(dir)
	if err != nil {
		t.Fatal(err)
	}

	tx := db.NewTx()
	defer tx.Rollback()

	// build a simple int-keyed leaf layout (block, id, data_value int)
	schema := record.NewSchema()
	schema.AddIntField("block")
	schema.AddIntField("id")
	schema.AddIntField("data_value")
	layout := record.NewLayout(schema)

	idx, err := btree.NewIndex(tx, "test_idx", layout)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	// insert 50 entries
	for i := 0; i < 50; i++ {
		rid := record.NewID(i/10, i%10)
		if err := idx.Insert(i, rid); err != nil {
			t.Fatalf("Insert(%d): %v", i, err)
		}
	}

	// search for key=25 — should find exactly one entry
	if err := idx.BeforeFirst(25); err != nil {
		t.Fatal(err)
	}
	found := 0
	for {
		has, err := idx.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !has {
			break
		}
		rid, err := idx.GetDataRecordID()
		if err != nil {
			t.Fatal(err)
		}
		if rid.BlockNumber() != 2 || rid.Slot() != 5 {
			t.Errorf("key=25: want RID(2,5), got RID(%d,%d)", rid.BlockNumber(), rid.Slot())
		}
		found++
	}
	if found != 1 {
		t.Errorf("key=25: want 1 result, got %d", found)
	}

	// delete key=25
	if err := idx.Delete(25, record.NewID(2, 5)); err != nil {
		t.Fatal(err)
	}

	// verify deleted
	if err := idx.BeforeFirst(25); err != nil {
		t.Fatal(err)
	}
	has, err := idx.Next()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("key=25 should be deleted but Next() returned true")
	}
}

func TestBTreeIndex_OrderedScan(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	db, err := server.NewGoDB(dir)
	if err != nil {
		t.Fatal(err)
	}

	tx := db.NewTx()
	defer tx.Rollback()

	schema := record.NewSchema()
	schema.AddIntField("block")
	schema.AddIntField("id")
	schema.AddIntField("data_value")
	layout := record.NewLayout(schema)

	idx, err := btree.NewIndex(tx, "ordered_idx", layout)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	// insert in reverse order to ensure tree sorts correctly
	for i := 99; i >= 0; i-- {
		if err := idx.Insert(i, record.NewID(i, 0)); err != nil {
			t.Fatalf("Insert(%d): %v", i, err)
		}
	}

	// search for key=50, verify correct RID
	if err := idx.BeforeFirst(50); err != nil {
		t.Fatal(err)
	}
	has, err := idx.Next()
	if err != nil || !has {
		t.Fatalf("BeforeFirst(50)+Next(): has=%v err=%v", has, err)
	}
	rid, _ := idx.GetDataRecordID()
	if rid.BlockNumber() != 50 {
		t.Errorf("want RID block=50, got %d", rid.BlockNumber())
	}
}
