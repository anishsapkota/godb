package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"godb/server"
)

const logo = "\033[36m" + `
  ____       ____  ____
 / ___|___  |  _ \| __ )
| |  / _ \ | | | |  _ \
| |_| (_) || |_| | |_) |
 \____\___/ |____/|____/
` + "\033[0m"

const version = "v0.1.0"

func main() {
	dir := "./godb-data"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	fmt.Print(logo)
	fmt.Printf("  \033[90mA relational database engine in Go · %s\033[0m\n", version)
	fmt.Printf("  \033[90mdata: %s\033[0m\n\n", dir)

	db, err := server.NewGoDB(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\033[90m  Type SQL followed by ; to execute. .quit to exit.\033[0m\n")

	scanner := bufio.NewScanner(os.Stdin)
	var buf strings.Builder
	fmt.Print("godb> ")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == ".quit" || line == ".exit" || line == "quit" || line == "exit" {
			break
		}
		if line == "" {
			continue
		}
		buf.WriteString(" ")
		buf.WriteString(line)

		stmt := strings.TrimSpace(buf.String())
		if !strings.HasSuffix(stmt, ";") {
			fmt.Print("   ...> ")
			continue
		}

		stmt = strings.TrimSuffix(stmt, ";")
		buf.Reset()

		runStmt(db, strings.TrimSpace(stmt))
		fmt.Print("godb> ")
	}
	fmt.Println("\nbye")
}

func runStmt(db *server.GoDB, stmt string) {
	tx := db.NewTx()
	lower := strings.ToLower(stmt)

	if strings.HasPrefix(lower, "select") {
		p, err := db.Planner().CreateQueryPlan(stmt, tx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			tx.Rollback()
			return
		}
		s := p.Open()
		fields := p.Schema().Fields()
		fmt.Println(strings.Join(fields, " | "))
		for {
			ok, err := s.Next()
			if err != nil || !ok {
				break
			}
			row := make([]string, len(fields))
			for i, f := range fields {
				v, _ := s.GetVal(f)
				row[i] = fmt.Sprintf("%v", v)
			}
			fmt.Println(strings.Join(row, " | "))
		}
		s.Close()
		if err := tx.Commit(); err != nil {
			fmt.Fprintf(os.Stderr, "commit error: %v\n", err)
		}
		return
	}

	n, err := db.Planner().ExecuteUpdate(stmt, tx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		tx.Rollback()
		return
	}
	fmt.Printf("%d rows affected\n", n)
	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "commit error: %v\n", err)
	}
}
