# Simplified Container Kit Makefile
# Focus on essential tasks after massive cleanup

# Version info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)

.PHONY: build cli mcp test test-integration fmt lint static-analysis security-scan arch-validate check-all clean version help wire-gen docs

# Primary build target - builds both CLI and MCP server
build: cli mcp
	@echo "✅ Build complete!"

cli:
	@echo "Building Container Kit CLI..."
	@echo "Version: $(VERSION)"
	GOFLAGS=-trimpath go build -ldflags "$(LDFLAGS)" -o container-kit ./main.go
	@echo "✅ Built: container-kit"

mcp:
	@echo "Building Container Kit MCP Server..."
	@echo "Version: $(VERSION)"
	GOFLAGS=-trimpath go build -tags mcp -ldflags "$(LDFLAGS)" -o container-kit-mcp ./cmd/mcp-server
	@echo "✅ Built: container-kit-mcp"

# Wire dependency injection code generation
wire-gen:
	@echo "Generating Wire dependency injection code..."
	@which wire > /dev/null || go install github.com/google/wire/cmd/wire@latest
	@cd pkg/mcp/composition && go generate
	@echo "✅ Wire code generated"

# Documentation generation
docs:
	@echo "Generating documentation..."
	@mkdir -p docs/architecture/diagrams/generated
	@if command -v plantuml >/dev/null 2>&1; then \
		echo "Generating PlantUML diagrams..."; \
		plantuml -tpng -o generated docs/architecture/diagrams/*.puml; \
		echo "✅ PlantUML diagrams generated in docs/architecture/diagrams/generated/"; \
	else \
		echo "⚠️ PlantUML not found. Install with: sudo apt-get install plantuml"; \
		echo "   Or use online: http://www.plantuml.com/plantuml/uml/"; \
	fi
	@if command -v mmdc >/dev/null 2>&1; then \
		echo "Generating Mermaid diagrams..."; \
		mmdc -i docs/architecture/diagrams/mcp_final_architecture.mmd -o docs/architecture/diagrams/generated/mcp_final_architecture.png; \
		echo "✅ Mermaid diagrams generated"; \
	else \
		echo "⚠️ Mermaid CLI not found. Install with: npm install -g @mermaid-js/mermaid-cli"; \
	fi
	@echo "✅ Documentation generation complete!"

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

arch-validate:
	@echo "Running architectural boundary validation..."
	@echo "Note: wiring/DI directories are excluded as they need to import from all layers"
	@cd scripts && go run arch-validate.go

perf-check:
	@echo "Running performance regression detection..."
	@go run scripts/perf/performance-monitor.go

contract-test:
	@echo "Running API contract tests..."
	@go test ./test/contract/ -v

check-all: fmt lint static-analysis security-scan arch-validate perf-check contract-test test

# Utility tasks  
clean:
	rm -f container-kit container-kit-mcp

version:
	@if [ -f "./container-kit-mcp" ]; then ./container-kit-mcp --version; else echo "❌ Run 'make build' first"; fi

help:
	@echo "Simplified Container Kit Makefile"
	@echo ""
	@echo "Essential targets:"
	@echo "  build             Build the MCP server binary"
	@echo "  wire-gen          Generate Wire dependency injection code"
	@echo "  docs              Generate architecture diagrams and documentation"
	@echo "  test              Run unit tests"
	@echo "  test-integration  Run integration tests"
	@echo "  fmt               Format code"
	@echo "  lint              Run linter"
	@echo "  static-analysis   Run static analysis (staticcheck)"
	@echo "  security-scan     Run security scanning (govulncheck)"
	@echo "  arch-validate     Run architecture boundary validation"
	@echo "  perf-check        Run performance regression detection"
	@echo "  contract-test     Run API contract stability tests"
	@echo "  check-all         Run all checks and tests (includes arch-validate, perf-check, contract-test)"
	@echo "  clean             Remove binaries"
	@echo "  version           Show binary version"
	@echo "  help              Show this help"