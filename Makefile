VENDOR_DIR = vendor

GO ?= go
GOLANGCI_LINT ?= golangci-lint

TEST_FLAGS = -race

ifeq ($(GOARCH), 386)
	TEST_FLAGS =
endif

.PHONY: $(VENDOR_DIR) lint test test-unit

$(VENDOR_DIR):
	@mkdir -p $(VENDOR_DIR)
	@$(GO) mod vendor
	@$(GO) mod tidy

lint:
	@$(GOLANGCI_LINT) run

test: test-unit

## Run unit tests
test-unit:
	@echo ">> unit test"
	@$(GO) test -gcflags=-l -coverprofile=unit.coverprofile -covermode=atomic $(TEST_FLAGS) ./...
