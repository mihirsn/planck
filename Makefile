GO = /opt/homebrew/bin/go
BINARY = planck
COVERAGE_OUT = coverage.out
COVERAGE_THRESHOLD = 90

.PHONY: all build run test lint coverage clean help

all: build

## build: compile the binary
build:
	$(GO) build -ldflags="-s -w" -o $(BINARY) .

## run: run planck with arguments (usage: make run ARGS="analyze app.log")
run:
	$(GO) run . $(ARGS)

## hooks: install git hooks (run once after cloning)
hooks:
	git config core.hooksPath .githooks
	@echo "✅ Git hooks installed. gofmt will run automatically on commit."

## fmt: format all Go source files
fmt:
	$(GO) fmt ./...

## test: run all unit tests
test:
	$(GO) test ./... -v

## coverage: run tests and generate coverage report (excludes cmd/ wiring)
coverage:
	$(GO) test ./internal/... -coverprofile=$(COVERAGE_OUT) -covermode=atomic
	$(GO) tool cover -html=$(COVERAGE_OUT) -o coverage.html
	@echo ""
	@$(GO) tool cover -func=$(COVERAGE_OUT) | grep total | awk '{print "Total coverage: " $$3}'

## coverage-check: fail if coverage of internal packages is below threshold
coverage-check:
	$(GO) test ./internal/... -coverprofile=$(COVERAGE_OUT) -covermode=atomic
	@COVERAGE=$$($(GO) tool cover -func=$(COVERAGE_OUT) | grep total | awk '{gsub(/%/, "", $$3); print int($$3)}'); \
	echo "Coverage: $$COVERAGE%"; \
	if [ "$$COVERAGE" -lt "$(COVERAGE_THRESHOLD)" ]; then \
		echo "❌ Coverage $$COVERAGE% is below the required $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	else \
		echo "✅ Coverage $$COVERAGE% meets the $(COVERAGE_THRESHOLD)% threshold"; \
	fi

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## clean: remove build artifacts
clean:
	rm -f $(BINARY) $(COVERAGE_OUT) coverage.html

## help: show this help message
help:
	@echo "Planck — available make targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
