# This Makefile contains targets for installing various development tools.
# The tools are installed in a local bin directory, making it easy to manage
# project-specific tool versions without affecting the system-wide installation.

TOOLS_DIR        ?= $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
TOOLS_MOD_FILE	:= $(TOOLS_DIR)/go.mod
BIN_DIR			?= $(TOOLS_DIR)/bin

# Detect OS and architecture
OS		:= $(shell uname -s | tr A-Z a-z)
ARCH	:= $(shell uname -m)

GOLANGCI_LINT_VERSION	?= $(shell grep github.com/golangci/golangci-lint $(TOOLS_MOD_FILE) | awk '{print $$3}')
GOLANGCI_LINT	:= $(BIN_DIR)/golangci-lint-$(OS)-$(ARCH)-$(GOLANGCI_LINT_VERSION)

$(GOLANGCI_LINT):
	$(call install-golangci-lint,$@,$(GOLANGCI_LINT_VERSION))

GOLANGCI_LINT_LINK 	:= $(BIN_DIR)/golangci-lint

.PHONY: $(GOLANGCI_LINT_LINK)
$(GOLANGCI_LINT_LINK): $(GOLANGCI_LINT)
	$(call create-symlink,$(GOLANGCI_LINT),$(GOLANGCI_LINT_LINK))

TOOLS := install-golangci-lint

.PHONY: install-tools
install-tools: $(TOOLS)

.PHONY: install-golangci-lint
install-golangci-lint: $(GOLANGCI_LINT) $(GOLANGCI_LINT_LINK)

.PHONY: clean-tools
clean-tools:
	rm -rf $(BIN_DIR)/*

# Update all tools
.PHONY: update-tools
update-tools: clean-tools install-tools

# go-install-tool installs a Go tool.
#
# $(1) binary path
# $(2) repo URL
# $(3) version
define go-install-tool
	@[ -f $(1) ] || { \
	set -e ;\
	TMP_DIR=$$(mktemp -d) ;\
	cd $$TMP_DIR ;\
	echo "Installing $(2)@$(3) to $(1)" ;\
	go mod init tmp ;\
	GOBIN=$$TMP_DIR go install $(2)@$(3) ;\
	mkdir -p $(dir $(1)) ;\
	mv $$TMP_DIR/$$(basename $(2)) $(1) ;\
	rm -rf $$TMP_DIR ;\
	}
endef

# install-golangci-lint installs golangci-lint.
#
# $(1) binary path
# $(2) version
define install-golangci-lint
	@[ -f $(1) ] || { \
	set -e ;\
	TMP_DIR=$$(mktemp -d) ;\
	cd $$TMP_DIR ;\
	echo "Installing golangci-lint $(2) to $(1)" ;\
	curl -fsSL -o install.sh https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh ;\
	chmod 0700 install.sh ;\
	./install.sh -b $$TMP_DIR $(2) ;\
	mkdir -p $(dir $(1)) ;\
	mv $$TMP_DIR/golangci-lint $(1) ;\
	rm -rf $$TMP_DIR ;\
	}
endef

# create-symlink creates a relative symlink to the platform-specific binary.
#
# $(1) platform-specific binary path
# $(2) symlink path
define create-symlink
	@if [ ! -e $(2) ] || [ "$$(readlink $(2))" != "$$(basename $(1))" ]; then \
		echo "Creating symlink: $(2) -> $$(basename $(1))"; \
		ln -sf $$(basename $(1)) $(2); \
	fi
endef