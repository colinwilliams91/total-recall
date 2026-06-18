BINARY_NAME = total-recall
BIN_DIR     = bin

ifeq ($(OS),Windows_NT)
    BINARY    = $(BIN_DIR)/$(BINARY_NAME).exe
    CLEAN_CMD = if exist $(BIN_DIR)\ rmdir /s /q $(BIN_DIR)
else
    BINARY    = $(BIN_DIR)/$(BINARY_NAME)
    CLEAN_CMD = rm -rf $(BIN_DIR)
endif

.PHONY: build install test lint clean tidy release-dry-run changelog

build:
	go build -o $(BINARY) ./cmd/total-recall

install:
	go install ./cmd/total-recall

test:
	go test ./...

lint:
	golangci-lint run

clean:
	$(CLEAN_CMD)

tidy:
	go mod tidy

release-dry-run:
	goreleaser release --snapshot --clean --skip=publish

changelog:
	git-cliff --output CHANGELOG.md
