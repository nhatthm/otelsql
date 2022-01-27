VENDOR_DIR = vendor

GO ?= go
GOLANGCI_LINT ?= golangci-lint
GHERKIN_LINT ?= gherkin-lint

TEST_FLAGS = -race
COMPATIBILITY_TEST ?= postgres

ifeq ($(GOARCH), 386)
	TEST_FLAGS =
endif

.PHONY: $(VENDOR_DIR) lint test test-unit test-compatibility

$(VENDOR_DIR):
	@mkdir -p $(VENDOR_DIR)
	@$(GO) mod vendor
	@$(GO) mod tidy

lint:
	@$(GOLANGCI_LINT) run
	@$(GHERKIN_LINT) -c tests/.gherkin-lintrc tests/features/*

test: test-unit

## Run unit tests
test-unit:
	@echo ">> unit test"
	@$(GO) test -gcflags=-l -coverprofile=unit.coverprofile -covermode=atomic $(TEST_FLAGS) ./...

test-compatibility:
	@echo ">> compatibility test"
	@echo ">> COMPATIBILITY_TEST = $(COMPATIBILITY_TEST)"
	@echo
	@cd "tests/$(COMPATIBILITY_TEST)"; $(GO) test -gcflags=-l -v ./...
