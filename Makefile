ROOT_PATH := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
COVERAGE_PATH := $(ROOT_PATH).coverage/

include $(CURDIR)/tools/tools.mk

.PHONY: lint
lint: install-golangci-lint
	@echo "Running linter..."
	$(GOLANGCI_LINT) run

.PHONY: vulncheck
vulncheck: install-govulncheck
	@echo "Checking for known vulnerabilities..."
	$(GOVULNCHECK) ./...

.PHONY: test
test:
	@echo "Running tests..."
	@go test ./... -count=1 -timeout=600s

.PHONY: test-cov
test-cov:
	@echo "Running tests with coverage..."
	@rm -rf $(COVERAGE_PATH)
	@mkdir -p $(COVERAGE_PATH)
	@go test -v -coverpkg=./... ./... -coverprofile $(COVERAGE_PATH)coverage.txt -count=1 -timeout=600s
	@go tool cover -func=$(COVERAGE_PATH)coverage.txt -o $(COVERAGE_PATH)functions.txt
	@go tool cover -html=$(COVERAGE_PATH)coverage.txt -o $(COVERAGE_PATH)coverage.html

.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@go test ./... -race -count=1 -timeout=600s

.PHONY: cross-compile-windows
cross-compile-windows:
	@echo "Cross-compiling tests for Windows..."
	@GOOS=windows GOARCH=amd64 \
	  go test ./... -run '^$$' -exec=true

.PHONY: verify
verify: lint vulncheck test-race cross-compile-windows
	@echo "All verifications passed successfully."

.PHONY: bench
bench:
	@go test -bench=. -benchmem -run='^$$' ./...
