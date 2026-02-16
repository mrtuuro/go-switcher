APP_NAME := switcher
BIN_DIR := ./bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)
MAIN_PKG := ./cmd/switcher

.PHONY: build install run test test-one lint fmt clean

build:
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_PATH) $(MAIN_PKG)

install: build
	@mkdir -p "$(HOME)/.switcher/bin"
	@cp $(BIN_PATH) "$(HOME)/.switcher/bin/switcher"
	@chmod +x "$(HOME)/.switcher/bin/switcher"
	@echo "installed switcher to $(HOME)/.switcher/bin/switcher"

run: build
	@$(BIN_PATH)

test:
	@go test -v ./...

test-one:
	@if [ -z "$(PKG)" ] || [ -z "$(TEST)" ]; then \
		echo "Usage: make test-one PKG=./internal/switcher TEST='^TestResolveActiveVersion$$'"; \
		exit 1; \
	fi
	@go test -v $(PKG) -run $(TEST)

lint:
	@golangci-lint run ./...

fmt:
	@gofmt -w ./cmd ./internal

clean:
	@rm -rf $(BIN_DIR)
