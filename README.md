# GoDB

A relational database engine implemented in Go, built from the ground up to understand database internals. Based on the concepts in [Database Design and Implementation](https://simpledb-java.netlify.app/database-design-and-implementation.pdf).

## Architecture

```
SQL string
    │
    ▼
parse/        — lexer + parser → AST (QueryData, InsertData, …)
    │
    ▼
plan/         — BasicQueryPlanner / BasicUpdatePlanner → Plan tree
    │
    ▼
query/scan/   — relational operators (Select, Project, Product scans)
    │
    ▼
record/       — schema, layout, record pages
    │
    ▼
tx/           — transactions (WAL, 2PL concurrency)
    │
    ▼
buffer/       — buffer pool (pin/unpin, LRU eviction)
    │
    ▼
file/         — block I/O (4096-byte blocks)
```

**Supporting subsystems:**
- `log/` — write-ahead log for crash recovery
- `metadata/` — catalog managers for tables, views, indexes, statistics
- `index/hash/` — hash index (bucket-per-hash, 100 buckets)
- `tx/concurrency/` — shared/exclusive lock table (block-level 2PL)

## SQL Support

```sql
-- Query
SELECT field1, field2 FROM table1, table2 WHERE field1 = 'value'

-- DML
INSERT INTO t (f1, f2) VALUES (1, 'hello')
DELETE FROM t WHERE f1 = 1
UPDATE t SET f1 = 2 WHERE f2 = 'hello'

-- DDL
CREATE TABLE t (id INT, name VARCHAR(32))
CREATE VIEW v AS SELECT id, name FROM t WHERE id > 0
CREATE INDEX idx ON t(id)
```

**Types:** `INT`, `LONG`, `SHORT`, `BOOLEAN`, `VARCHAR(n)`, `DATE`

**Predicates:** `=`, `<>`, `<`, `>`, `<=`, `>=`

## Getting Started

```bash
# Build
make build

# Run REPL (default data dir: ./godb-data)
make run

# Run with custom data directory
make run DATA=/path/to/data

# Tests
make test
make test-v   # verbose
```

Or directly:

```bash
go build -o godb ./cmd/godb
./godb [data-dir]
```

### REPL Example

```
  ____       ____  ____
 / ___|___  |  _ \| __ )
| |  / _ \ | | | |  _ \
| |_| (_) || |_| | |_) |
 \____\___/ |____/|____/

  A relational database engine in Go · v0.1.0
  data: ./godb-data

creating new database
  Type SQL followed by ; to execute. .quit to exit.

godb> CREATE TABLE students (id INT, name VARCHAR(64), gpa LONG);
0 rows affected
godb> INSERT INTO students (id, name, gpa) VALUES (1, 'Alice', 390);
1 rows affected
godb> SELECT id, name FROM students WHERE gpa > 360;
id | name
1 | Alice
```

## Project Structure

```
godb/
├── buffer/          buffer pool manager + replacement strategies
├── cmd/godb/        REPL entry point
├── file/            block-level file I/O
├── index/hash/      hash index implementation
├── log/             write-ahead log
├── metadata/        table / view / index / stat catalogs
├── parse/           SQL lexer and parser
├── plan/            query and update planners
├── query/
│   ├── scan/        Scan interface (relational operators)
│   └── table/       TableScan
├── record/          schema, layout, record page marshaling
├── server/          GoDB orchestrator (wires all components)
├── tx/
│   └── concurrency/ lock table, concurrency manager
└── utils/           shared utilities
```

## Key Design Decisions

- **Block size:** 4096 bytes (fits catalog slot overhead)
- **Query planning:** left-deep product trees with select/project wrappers
- **Recovery:** WAL with undo-only recovery on startup
- **Locking:** block-level shared/exclusive locks (strict 2PL)
- **Indexes:** hash index; pluggable via `index.Index` interface
- **Buffer eviction:** naive (first unpinned) and LRU strategies

## Dependencies

- Go 1.23+
- [`github.com/stretchr/testify`](https://github.com/stretchr/testify) — test assertions only
