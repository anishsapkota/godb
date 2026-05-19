GO     := $(shell command -v go 2>/dev/null || echo /usr/local/go/bin/go)
BIN    := mydb
DATA   := ./mydb-data

.PHONY: help build run test clean

## help: show this help
help:
	@echo "Usage: make <target>"
	@echo ""
	@awk '/^##/ { sub(/^## /, ""); split($$0, a, ": "); printf "  \033[36m%-12s\033[0m %s\n", a[1], a[2] }' $(MAKEFILE_LIST)

## build: compile binary to ./mydb
build:
	$(GO) build -o $(BIN) ./cmd/mydb

## run: run REPL against DATA dir (override: make run DATA=/path)
run: build
	./$(BIN) $(DATA)

## test: run all tests
test:
	$(GO) test ./... -timeout 60s

## test-v: run all tests verbose
test-v:
	$(GO) test ./... -timeout 60s -v

## clean: remove binary and data dir
clean:
	rm -f $(BIN)
	rm -rf $(DATA)
