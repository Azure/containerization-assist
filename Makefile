# Container Kit Makefile

# Get version from git
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)
GOFLAGS := -trimpath

.PHONY: build
build: mcp

.PHONY: mcp
mcp:
	@echo "Building Container Kit MCP Server..."
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	GOFLAGS=$(GOFLAGS) go build -tags mcp -ldflags "$(LDFLAGS)" -o container-kit-mcp ./cmd/mcp-server
	@echo "Build complete: container-kit-mcp"
	@echo ""
	@echo "Test version flag: ./container-kit-mcp --version"

.PHONY: build-mcp
build-mcp: mcp

.PHONY: test
test:
	go test -race ./pkg/mcp/...

.PHONY: test-mcp
test-mcp:
	go test -tags mcp -race ./pkg/mcp/...

.PHONY: test-all
test-all:
	go test -race ./...

.PHONY: bench
bench:
	@echo "Running MCP performance benchmarks..."
	@echo "Target: <300μs P95 per request"
	go test -bench=. -benchmem -benchtime=5s ./pkg/mcp/tools

.PHONY: bench-baseline
bench-baseline:
	@echo "Setting performance baseline..."
	go test -bench=. -benchmem -benchtime=10s ./pkg/mcp/tools > bench-baseline.txt
	@echo "Baseline saved to bench-baseline.txt"

.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "❌ golangci-lint not found. Install with:"; echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.55.2"; echo "  Or use the development container: see .devcontainer/README.md"; exit 1)
	@echo "Running linter with error budget (threshold: 100)..."
	@LINT_ERROR_THRESHOLD=100 LINT_WARN_THRESHOLD=50 ./scripts/lint-with-threshold.sh ./pkg/mcp/...

.PHONY: lint-strict
lint-strict:
	@which golangci-lint > /dev/null || (echo "❌ golangci-lint not found. Install with:"; echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.55.2"; echo "  Or use the development container: see .devcontainer/README.md"; exit 1)
	@echo "Running linter in strict mode (all issues)..."
	golangci-lint run ./pkg/mcp/...

.PHONY: lint-all
lint-all:
	@which golangci-lint > /dev/null || (echo "❌ golangci-lint not found. Install with:"; echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.55.2"; echo "  Or use the development container: see .devcontainer/README.md"; exit 1)
	golangci-lint run ./...

.PHONY: clean
clean:
	rm -f container-kit-mcp

.PHONY: deps-update
deps-update:
	@echo "Updating Go dependencies..."
	@echo "Current go.mod version: $$(go version | cut -d' ' -f3)"
	@echo ""
	@echo "Fetching latest versions..."
	go get -u ./...
	@echo ""
	@echo "Tidying go.mod..."
	go mod tidy
	@echo ""
	@echo "Verifying dependencies..."
	go mod verify
	@echo ""
	@echo "Testing with updated dependencies..."
	go test ./...
	@echo ""
	@echo "Dependencies updated successfully!"
	@echo ""
	@echo "To commit these changes, run:"
	@echo "  git add go.mod go.sum"
	@echo "  git commit -m 'deps: update Go dependencies'"
	@echo ""
	@echo "Or use the automated changelog commit:"
	@echo "  make deps-commit"

.PHONY: deps-commit
deps-commit:
	@echo "Creating dependency update commit with changelog template..."
	@if ! git diff --quiet go.mod go.sum; then \
		echo ""; \
		echo "📦 Dependency Update Summary"; \
		echo ""; \
		echo "Updated packages:"; \
		git diff go.mod | grep "^+" | grep -v "^+++" | grep -E "^\+\s+[a-zA-Z]" | sed 's/^+/  -/' || echo "  - See go.mod diff for details"; \
		echo ""; \
		echo "Committing changes..."; \
		git add go.mod go.sum; \
		git commit -m "deps: update Go dependencies"; \
		echo ""; \
		echo "✅ Dependency update committed successfully!"; \
	else \
		echo "❌ No changes detected in go.mod or go.sum"; \
		echo "Run 'make deps-update' first to update dependencies"; \
		exit 1; \
	fi

.PHONY: version
version:
	@if [ ! -f "./container-kit-mcp" ]; then echo "❌ Binary not found. Run 'make mcp' first."; exit 1; fi
	@./container-kit-mcp --version

.PHONY: dev-mcp
dev-mcp: build-mcp
	@echo "MCP server built successfully"

.PHONY: lint-report
lint-report:
	@echo "Generating comprehensive lint report..."
	@./scripts/lint-with-threshold.sh ./pkg/mcp/... || true
	@echo ""
	@echo "Run 'make lint-threshold' to check against error budget"

.PHONY: lint-baseline
lint-baseline:
	@echo "Setting current lint count as baseline..."
	@golangci-lint run --out-format json ./pkg/mcp/... 2>/dev/null | jq '.Issues | length' > .lint-baseline || echo "0" > .lint-baseline
	@echo "Baseline set to: $$(cat .lint-baseline) issues"

.PHONY: lint-ratchet
lint-ratchet:
	@./scripts/lint-ratchet.sh ./pkg/mcp/...

.PHONY: help
help:
	@echo "Container Kit Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build, mcp        Build the MCP server binary"
	@echo "  test              Run MCP package tests"
	@echo "  test-mcp          Run MCP tests with build tags"
	@echo "  test-all          Run all tests"
	@echo "  bench             Run performance benchmarks (target: <300μs P95)"
	@echo "  bench-baseline    Create performance baseline"
	@echo ""
	@echo "Linting targets:"
	@echo "  lint              Run linting with error budget (threshold: 100 issues)"
	@echo "  lint-strict       Run linting in strict mode (shows all issues)"
	@echo "  lint-report       Generate detailed lint report"
	@echo "  lint-baseline     Set current issue count as baseline"
	@echo "  lint-ratchet      Ensure issues don't increase from baseline"
	@echo "  lint-all          Run linting on all packages (strict mode)"
	@echo ""
	@echo "Dependency management:"
	@echo "  deps-update       Update Go dependencies (go get -u && go mod tidy)"
	@echo "  deps-commit       Commit dependency updates with changelog template"
	@echo ""
	@echo "Other targets:"
	@echo "  clean             Remove built binaries"
	@echo "  version           Show version of built binary"
	@echo "  help              Show this help message"
	@echo ""
	@echo "Lint thresholds:"
	@echo "  Error: 100 issues (build fails above this)"
	@echo "  Warning: 50 issues (warning above this)"
	@echo ""
	@echo "To check current lint issues: make lint-report"