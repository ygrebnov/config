ROOT_PATH := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
COVERAGE_PATH := $(ROOT_PATH).coverage/

include $(CURDIR)/tools/tools.mk

.PHONY: lint
lint: install-golangci-lint
	$(GOLANGCI_LINT) run

.PHONY: test
test:
	@rm -rf $(COVERAGE_PATH)
	@mkdir -p $(COVERAGE_PATH)
	@go test -v -coverpkg=./... ./... -coverprofile $(COVERAGE_PATH)coverage.txt
	@go tool cover -func=$(COVERAGE_PATH)coverage.txt -o $(COVERAGE_PATH)functions.txt
	@go tool cover -html=$(COVERAGE_PATH)coverage.txt -o $(COVERAGE_PATH)coverage.html

.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@go clean -testcache
	@go test ./... -race -count=1 -timeout=600s

.PHONY: bench
bench:
	@go test -bench=. -benchmem -run=^$
