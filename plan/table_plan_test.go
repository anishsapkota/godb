package plan_test

import (
	"path/filepath"
	"testing"

	"godb/plan"
	"godb/query/table"
	"godb/record"
	"godb/server"
)

func TestTablePlan_OpenAndCount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	db, err := server.NewGoDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.NewTx()
	defer tx.Rollback()

	sch := record.NewSchema()
	sch.AddIntField("id")
	sch.AddStringField("name", 20)

	if err := db.MetadataManager().CreateTable("students", sch, tx); err != nil {
		t.Fatal(err)
	}

	layout, err := db.MetadataManager().GetLayout("students", tx)
	if err != nil {
		t.Fatal(err)
	}

	ts, err := table.NewTableScan(tx, "students", layout)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if err := ts.Insert(); err != nil {
			t.Fatal(err)
		}
		if err := ts.SetInt("id", i); err != nil {
			t.Fatal(err)
		}
		if err := ts.SetString("name", "row"); err != nil {
			t.Fatal(err)
		}
	}
	ts.Close()

	tp, err := plan.NewTablePlan(tx, "students", db.MetadataManager())
	if err != nil {
		t.Fatal(err)
	}

	s := tp.Open()
	count := 0
	for {
		ok, err := s.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		count++
	}
	s.Close()

	if count != 3 {
		t.Errorf("want 3 rows, got %d", count)
	}
	if tp.RecordsOutput() <= 0 {
		t.Errorf("RecordsOutput should be positive, got %d", tp.RecordsOutput())
	}
}
