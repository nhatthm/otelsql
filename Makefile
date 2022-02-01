VENDOR_DIR = vendor

GO ?= go
GOLANGCI_LINT ?= golangci-lint
GHERKIN_LINT ?= gherkin-lint

TEST_FLAGS = -race
COMPATIBILITY_TEST ?= postgres

ifeq ($(GOARCH), 386)
	TEST_FLAGS =
endif

goModules := $(shell find . -name 'go.mod' | xargs dirname)
lintGoModules := $(subst -.,-module,$(subst /,-,$(addprefix lint-,$(goModules))))
compatibilityTests := $(addprefix test-compatibility-,$(filter-out suite,$(subst ./,,$(shell cd tests;find . -name 'go.mod' | xargs dirname))))

.PHONY: $(VENDOR_DIR) $(lintGoModules) $(compatibilityTests) lint test test-unit test-compatibility

$(VENDOR_DIR):
	@mkdir -p $(VENDOR_DIR)
	@$(GO) mod vendor
	@$(GO) mod tidy

$(lintGoModules):
	$(eval GO_MODULE := "$(subst lint/module,.,$(subst -,/,$(subst lint-module-,,$@)))")

	@echo ">> module: $(GO_MODULE)"
	@cd "$(GO_MODULE)"; $(GOLANGCI_LINT) run

lint: $(lintGoModules)

test: test-unit test-compatibility

## Run unit tests
test-unit:
	@echo ">> unit test"
	@$(GO) test -gcflags=-l -coverprofile=unit.coverprofile -covermode=atomic $(TEST_FLAGS) ./...
	@echo

$(compatibilityTests):
	$(eval COMPATIBILITY_TEST := "$(subst test-compatibility-,,$@)")
	@echo ">> compatibility test: $(COMPATIBILITY_TEST)"
	@cd "tests/$(COMPATIBILITY_TEST)"; $(GO) test -gcflags=-l -v ./...
	@echo

test-compatibility: $(compatibilityTests)
