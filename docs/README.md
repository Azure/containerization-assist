# Container Security Scanning Framework Documentation

This directory contains comprehensive documentation for the container security scanning framework built as part of Workstream C: Security & External Integrations.

## 📚 Documentation Overview

### Core Documentation
- [**Security Scanning Workflow**](security-scanning-workflow.md) - Complete guide to using the security scanning framework
- [**API Reference**](api-reference.md) - Detailed API documentation for all components

### Quick Links
- [Quick Start](#quick-start)
- [Architecture Overview](#architecture)
- [Examples](#examples)
- [Contributing](#contributing)

## 🚀 Quick Start

### Installation

1. **Install vulnerability scanners**:
```bash
# Install Trivy
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin

# Install Grype
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin
```

2. **Set up Go environment**:
```bash
go mod tidy
```

3. **Optional: Configure NIST API key**:
```bash
export NIST_API_KEY="your-nist-api-key"
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/rs/zerolog"
    "github.com/Azure/container-kit/pkg/core/docker"
)

func main() {
    logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
    
    // Create unified scanner
    scanner := docker.NewUnifiedSecurityScanner(logger)
    
    // Scan image
    result, err := scanner.ScanImage(context.Background(), "nginx:latest", "HIGH,CRITICAL")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Found %d vulnerabilities\n", result.CombinedSummary.Total)
    fmt.Printf("Scanner agreement: %.1f%%\n", result.ComparisonMetrics.AgreementRate)
}
```

## 🏗 Architecture

The security scanning framework consists of several key components:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Security Scanning Framework                  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   Trivy Scanner │  │  Grype Scanner  │  │ Secret Discovery│  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│           │                     │                     │         │
│           └─────────────────────┼─────────────────────┘         │
│                                 │                               │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │              Unified Security Scanner                       │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                 │                               │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │              Policy Engine                                  │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                 │                               │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │              CVE Database Integration                       │  │
│  └─────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Components

1. **[Vulnerability Scanners](../pkg/core/docker/)**: Trivy, Grype, and unified scanning
2. **[Policy Engine](../pkg/core/security/policy_engine.go)**: Flexible security policy enforcement
3. **[Secret Discovery](../pkg/core/security/secret_discovery.go)**: Pattern and entropy-based secret detection
4. **[CVE Database](../pkg/core/security/cve_database.go)**: NIST National Vulnerability Database integration
5. **[Registry Health](../pkg/core/docker/registry_health.go)**: Docker registry monitoring

## 📖 Examples

### Vulnerability Scanning

```go
// Single scanner usage
scanner := docker.NewTrivyScanner(logger)
result, err := scanner.ScanImage(ctx, "nginx:latest", "HIGH,CRITICAL")

// Unified scanning with multiple scanners
unifiedScanner := docker.NewUnifiedSecurityScanner(logger)
result, err := unifiedScanner.ScanImage(ctx, "nginx:latest", "HIGH,CRITICAL")
```

### Policy Enforcement

```go
// Create policy engine
policyEngine := security.NewPolicyEngine(logger)
err := policyEngine.LoadDefaultPolicies()

// Evaluate policies against scan results
results, err := policyEngine.EvaluatePolicies(ctx, scanContext)
if policyEngine.ShouldBlock(results) {
    // Handle policy violations
}
```

### Secret Detection

```go
// Create secret discovery engine
discovery := security.NewSecretDiscovery(logger)

// Scan directory for secrets
findings, err := discovery.ScanDirectory("/path/to/project", nil)
summary := discovery.GenerateSummary(findings)
```

### CVE Database Integration

```go
// Create CVE database client
cveDB := security.NewCVEDatabase(logger)

// Get detailed CVE information
cveRecord, err := cveDB.GetCVE(ctx, "CVE-2023-1234")

// Enrich vulnerability with additional data
err := cveDB.EnrichVulnerability(ctx, &vulnerability)
```

## 🧪 Testing

### Run Tests

```bash
# Run all security tests
go test ./pkg/core/security -v

# Run tests with coverage
go test ./pkg/core/security -cover

# Run docker scanner tests
go test ./pkg/core/docker -v
```

### Integration Tests

```bash
# Run integration tests (requires scanners installed)
go test ./pkg/core/... -tags=integration -v
```

## 🔧 Configuration

### Environment Variables

```bash
# CVE Database
export NIST_API_KEY="your-api-key"
export CVE_CACHE_TTL="24h"

# Scanner Configuration
export TRIVY_DB_REPOSITORY="ghcr.io/aquasecurity/trivy-db"
export GRYPE_DB_AUTO_UPDATE="true"

# Logging
export LOG_LEVEL="info"
export LOG_FORMAT="json"
```

### Custom Policies

Create security policies in YAML format:

```yaml
# security-policies.yaml
policies:
  - id: "critical-vulns-block"
    name: "Block Critical Vulnerabilities"
    enabled: true
    severity: "critical"
    rules:
      - id: "no-critical"
        type: "vulnerability_count"
        field: "critical"
        operator: "greater_than"
        value: 0
    actions:
      - type: "block"
      - type: "notify"
```

## 🚀 Integration Patterns

### CI/CD Integration

#### GitHub Actions

```yaml
name: Security Scan
on: [push, pull_request]

jobs:
  security:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - name: Security scan
      run: |
        go run ./cmd/security-scanner \
          --image myapp:${{ github.sha }} \
          --severity HIGH,CRITICAL \
          --fail-on-violation
```

### Kubernetes Integration

```go
// Admission controller webhook
func (w *SecurityWebhook) ValidateImage(ctx context.Context, image string) error {
    scanner := docker.NewUnifiedSecurityScanner(w.logger)
    result, err := scanner.ScanImage(ctx, image, "HIGH,CRITICAL")
    if err != nil {
        return fmt.Errorf("scan failed: %w", err)
    }
    
    if w.policyEngine.ShouldBlock(result) {
        return fmt.Errorf("image blocked by security policies")
    }
    
    return nil
}
```

## 🤝 Contributing

### Development Setup

1. Clone the repository
2. Install dependencies: `go mod download`
3. Install dev tools: `make dev-setup`
4. Run tests: `make test`

### Code Standards

- Follow Go conventions and use `gofmt`
- Add tests for new features
- Update documentation for API changes
- Maintain test coverage above 80%

### Pull Request Process

1. Create feature branch from `main`
2. Make changes with comprehensive tests
3. Run `make test lint` to verify quality
4. Update documentation as needed
5. Submit pull request with clear description

## 📊 Monitoring

### Health Checks

```go
// Check scanner availability
trivyAvailable := scanner.CheckTrivyInstalled()
grypeAvailable := scanner.CheckGrypeInstalled()

// Check registry health
health, err := healthChecker.CheckHealth(ctx, registryURL, auth)
```

### Metrics

```go
// CVE database metrics
stats := cveDB.GetCacheStats()
fmt.Printf("Cache hit rate: %.2f%%\n", stats.HitRate)

// Scanner performance
fmt.Printf("Scan duration: %v\n", result.Duration)
fmt.Printf("Agreement rate: %.1f%%\n", result.ComparisonMetrics.AgreementRate)
```

## 🔒 Security Considerations

- Always validate input parameters
- Use secure defaults for configurations
- Regularly update scanner databases
- Monitor for new vulnerability patterns
- Follow principle of least privilege
- Implement proper error handling

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/Azure/container-kit/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Azure/container-kit/discussions)
- **Security**: Follow responsible disclosure process

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.

---

For detailed information, see the complete [Security Scanning Workflow Guide](security-scanning-workflow.md) and [API Reference](api-reference.md).