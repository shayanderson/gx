.ONESHELL:
.DEFAULT_GOAL := help

.PHONY: help
help: ## Display help
	@MKH_COL_W=20
	@grep -h '##' $(MAKEFILE_LIST) | \
	  grep -v grep | grep -v MKH_COL_W | sort | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-'$$MKH_COL_W's\033[0m %s\n", $$1, $$2}' | \
	  sort

.PHONY: test
test: ## Run tests
	go test -race -timeout 15s ./...

.PHONY: test-bench
test-bench: ## Run tests with benchmarks
	go test -bench=. -benchmem ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage
	go test -race -timeout 15s -coverprofile=/tmp/testcoverage.txt ./...
	go tool cover -html=/tmp/testcoverage.txt
