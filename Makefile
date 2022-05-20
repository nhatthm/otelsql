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
tidyGoModules := $(subst -.,-module,$(subst /,-,$(addprefix tidy-,$(goModules))))
updateGoModules := $(subst -.,-module,$(subst /,-,$(addprefix update-,$(goModules))))
lintGoModules := $(subst -.,-module,$(subst /,-,$(addprefix lint-,$(goModules))))
compatibilityTests := $(addprefix test-compatibility-,$(filter-out suite,$(subst ./,,$(shell cd tests;find . -name 'go.mod' | xargs dirname))))

.PHONY: $(VENDOR_DIR)
$(VENDOR_DIR):
	@mkdir -p $(VENDOR_DIR)
	@$(GO) mod tidy
	@$(GO) mod vendor

.PHONY: $(lintGoModules)
$(lintGoModules):
	$(eval GO_MODULE := "$(subst lint/module,.,$(subst -,/,$(subst lint-module-,,$@)))")

	@echo ">> module: $(GO_MODULE)"
	@cd "$(GO_MODULE)"; $(GOLANGCI_LINT) run

.PHONY: lint
lint: $(lintGoModules)

.PHONY: $(tidyGoModules)
$(tidyGoModules):
	$(eval GO_MODULE := "$(subst tidy/module,.,$(subst -,/,$(subst tidy-module-,,$@)))")

	@echo ">> module: $(GO_MODULE)"
	@cd "$(GO_MODULE)"; $(GO) mod tidy -compat=1.17

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
