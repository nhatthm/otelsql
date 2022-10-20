MODULE_NAME=otelsql

VENDOR_DIR = vendor

GOLANGCI_LINT_VERSION ?= v1.50.0

GO ?= go
GOLANGCI_LINT ?= $(shell go env GOPATH)/bin/golangci-lint-$(GOLANGCI_LINT_VERSION)
GHERKIN_LINT ?= gherkin-lint

TEST_FLAGS = -race
COMPATIBILITY_TEST ?= postgres

ifeq ($(GOARCH), 386)
	TEST_FLAGS =
endif

goModules := $(shell find . -name 'go.mod' | xargs dirname)
tidyGoModules := $(subst -.,-module,$(subst /,-,$(addprefix tidy-,$(goModules))))
updateGoModules := $(subst -.,-module,$(subst /,-,$(addprefix update-,$(goModules))))
lintGoModules := $(subst -.,-module,$(subst /,-,$(addprefix lint-,$(goModules))))
compatibilityTests := $(addprefix test-compatibility-,$(filter-out suite,$(subst ./,,$(shell cd tests;find . -name 'go.mod' | xargs dirname))))

.PHONY: help
help:
	@make -qpRr | egrep -e '^[a-z].*:$$' | sed -e 's~:~~g' | sort

.PHONY: $(VENDOR_DIR)
$(VENDOR_DIR):
	@mkdir -p $(VENDOR_DIR)
	@$(GO) mod tidy
	@$(GO) mod vendor

.PHONY: $(lintGoModules)
$(lintGoModules): $(GOLANGCI_LINT)
	$(eval GO_MODULE := "$(subst lint/module,.,$(subst -,/,$(subst lint-module-,,$@)))")

	@echo ">> module: $(GO_MODULE)"
	@cd "$(GO_MODULE)"; $(GOLANGCI_LINT) run

.PHONY: lint
lint: $(lintGoModules)

.PHONY: $(tidyGoModules)
$(tidyGoModules):
	$(eval GO_MODULE := "$(subst tidy/module,.,$(subst -,/,$(subst tidy-module-,,$@)))")

	@echo ">> module: $(GO_MODULE)"
	@cd "$(GO_MODULE)"; $(GO) mod tidy

.PHONY: tidy
tidy: $(tidyGoModules)

.PHONY: $(updateGoModules)
$(updateGoModules):
	$(eval GO_MODULE := "$(subst update/module,.,$(subst -,/,$(subst update-module-,,$@)))")

	@echo ">> module: $(GO_MODULE)"
	@cd "$(GO_MODULE)"; $(GO) get -u ./...

.PHONY: update
update: $(updateGoModules)

## Run unit tests
.PHONY: test-unit
test-unit:
	@echo ">> unit test"
	@$(GO) test -gcflags=-l -coverprofile=unit.coverprofile -covermode=atomic $(TEST_FLAGS) ./...
	@echo

.PHONY: $(compatibilityTests)
$(compatibilityTests):
	$(eval COMPATIBILITY_TEST := "$(subst test-compatibility-,,$@)")
	@echo ">> compatibility test: $(COMPATIBILITY_TEST)"
	@cd "tests/$(COMPATIBILITY_TEST)"; $(GO) test -gcflags=-l -v ./...
	@echo

.PHONY: test-compatibility
test-compatibility: $(compatibilityTests)

.PHONY: test
test: test-unit test-compatibility

.PHONY: gha-vars
gha-vars:
	@echo "::set-output name=MODULE_NAME::$(MODULE_NAME)"
	@echo "::set-output name=GOLANGCI_LINT_VERSION::$(GOLANGCI_LINT_VERSION)"

$(GOLANGCI_LINT):
	@echo "$(OK_COLOR)==> Installing golangci-lint $(GOLANGCI_LINT_VERSION)$(NO_COLOR)"; \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./bin "$(GOLANGCI_LINT_VERSION)"
	@mv ./bin/golangci-lint $(GOLANGCI_LINT)
