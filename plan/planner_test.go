package plan_test

import (
	"path/filepath"
	"testing"

	"godb/server"
)

func TestPlanner_RoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	db, err := server.NewGoDB(dir)
	if err != nil {
		t.Fatal(err)
	}

	exec := func(sql string) int {
		tx := db.NewTx()
		n, err := db.Planner().ExecuteUpdate(sql, tx)
		if err != nil {
			t.Fatalf("ExecuteUpdate(%q): %v", sql, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		return n
	}

	exec("create table students (id int, name varchar(20))")
	if n := exec("insert into students (id, name) values (1, 'ada')"); n != 1 {
		t.Errorf("insert: want 1, got %d", n)
	}
	if n := exec("insert into students (id, name) values (2, 'linus')"); n != 1 {
		t.Errorf("insert: want 1, got %d", n)
	}

	tx := db.NewTx()
	p, err := db.Planner().CreateQueryPlan("select id, name from students", tx)
	if err != nil {
		t.Fatal(err)
	}
	s := p.Open()
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
	tx.Commit()

	if count != 2 {
		t.Errorf("select all: want 2 rows, got %d", count)
	}

	tx2 := db.NewTx()
	p2, err := db.Planner().CreateQueryPlan("select name from students where id = 2", tx2)
	if err != nil {
		t.Fatal(err)
	}
	s2 := p2.Open()
	count2 := 0
	var lastName string
	for {
		ok, err := s2.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		v, _ := s2.GetVal("name")
		lastName = v.(string)
		count2++
	}
	s2.Close()
	tx2.Commit()

	if count2 != 1 {
		t.Errorf("select where: want 1 row, got %d", count2)
	}
	if lastName != "linus" {
		t.Errorf("want 'linus', got %q", lastName)
	}
}
