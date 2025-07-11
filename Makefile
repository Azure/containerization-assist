# Simplified Container Kit Makefile
# Focus on essential tasks after massive cleanup

# Version info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)

.PHONY: build mcp test test-integration fmt lint static-analysis security-scan check-all clean version help

# Primary build target
build: mcp
mcp:
	@echo "Building Container Kit MCP Server..."
	@echo "Version: $(VERSION)"
	GOFLAGS=-trimpath go build -tags mcp -ldflags "$(LDFLAGS)" -o container-kit-mcp ./cmd/mcp-server
	@echo "✅ Build complete: container-kit-mcp"

# Essential development tasks
test:
	@echo "Running tests..."
	go test -race ./pkg/mcp/... ./pkg/core/...

test-integration:
	@echo "Running integration tests..."
	@./test/integration/run_tests.sh

fmt:
	@echo "Formatting code..."
	@gofmt -s -w .
	@go mod tidy
	@echo "✅ Formatting complete"

lint:
	@echo "Running linter..."
	@which $$(go env GOPATH)/bin/golangci-lint > /dev/null || (echo "Install golangci-lint: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest"; exit 1)
	$$(go env GOPATH)/bin/golangci-lint run ./pkg/mcp/... ./pkg/core/...

static-analysis:
	@echo "Running static analysis..."
	@which $$(go env GOPATH)/bin/staticcheck > /dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
	$$(go env GOPATH)/bin/staticcheck ./pkg/mcp/... ./pkg/core/...

security-scan:
	@echo "Running security scan..."
	@which $$(go env GOPATH)/bin/govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	$$(go env GOPATH)/bin/govulncheck ./...

check-all: fmt lint static-analysis security-scan test

# Utility tasks  
clean:
	rm -f container-kit-mcp

version:
	@if [ -f "./container-kit-mcp" ]; then ./container-kit-mcp --version; else echo "❌ Run 'make build' first"; fi

help:
	@echo "Simplified Container Kit Makefile"
	@echo ""
	@echo "Essential targets:"
	@echo "  build             Build the MCP server binary"
	@echo "  test              Run unit tests"
	@echo "  test-integration  Run integration tests"
	@echo "  fmt               Format code"
	@echo "  lint              Run linter"
	@echo "  static-analysis   Run static analysis (staticcheck)"
	@echo "  security-scan     Run security scanning (govulncheck)"
	@echo "  check-all         Run all checks and tests"
	@echo "  clean             Remove binaries"
	@echo "  version           Show binary version"
	@echo "  help              Show this help"