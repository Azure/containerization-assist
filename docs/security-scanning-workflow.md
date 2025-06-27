# Security Scanning Workflow and Integration

This document provides comprehensive guidance on integrating and using the container security scanning framework built for Workstream C: Security & External Integrations.

## Overview

The security scanning framework provides:
- **Multi-scanner support**: Trivy and Grype vulnerability scanners
- **Secret detection**: Pattern-based and entropy-based secret discovery
- **CVE database integration**: NIST National Vulnerability Database (NVD) integration
- **Policy enforcement**: Flexible security policy engine with customizable rules
- **Registry health checks**: Docker registry health monitoring

## Architecture

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

## Components

### 1. Vulnerability Scanners

#### Trivy Scanner (`pkg/core/docker/trivy.go`)
- **Purpose**: Industry-standard vulnerability scanner for containers
- **Features**:
  - Comprehensive vulnerability detection
  - CVSS v2/v3 scoring
  - CWE (Common Weakness Enumeration) mapping
  - Layer-based analysis
  - Package metadata extraction

#### Grype Scanner (`pkg/core/docker/grype.go`)
- **Purpose**: Anchore's vulnerability scanner for containers
- **Features**:
  - Alternative vulnerability detection engine
  - Cross-reference with Trivy for comprehensive coverage
  - Package manager support (npm, pip, gem, etc.)
  - CPE (Common Platform Enumeration) matching

#### Unified Scanner (`pkg/core/docker/unified_scanner.go`)
- **Purpose**: Combines multiple scanners for comprehensive analysis
- **Features**:
  - Parallel scanner execution
  - Result deduplication and merging
  - Scanner agreement metrics
  - Fallback support if one scanner fails

### 2. Secret Detection (`pkg/core/security/secret_discovery.go`)
- **Pattern-based detection**: Regex patterns for known secret types
- **Entropy-based detection**: High-entropy string detection
- **False positive filtering**: Reduces noise from test/example data
- **Verification**: Optional verification of detected secrets

### 3. Policy Engine (`pkg/core/security/policy_engine.go`)
- **Flexible rule system**: Multiple rule types and operators
- **Policy categories**: Vulnerability, Secret, Compliance, Image, Configuration
- **Action system**: Block, Warn, Log, Notify, Quarantine, Auto-fix
- **Default policies**: Pre-configured security policies

### 4. CVE Database Integration (`pkg/core/security/cve_database.go`)
- **NIST NVD API**: Direct integration with National Vulnerability Database
- **Caching**: Local caching for performance
- **Enrichment**: Enhance vulnerability data with additional details
- **Search capabilities**: Query CVEs by various criteria

## Quick Start

### Installation

1. **Install scanners**:
```bash
# Install Trivy
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin

# Install Grype
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin
```

2. **Import the package**:
```go
import (
    "github.com/Azure/container-kit/pkg/core/docker"
    "github.com/Azure/container-kit/pkg/core/security"
)
```

### Basic Usage

#### 1. Single Scanner Usage

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

    // Create scanner
    scanner := docker.NewTrivyScanner(logger)

    // Scan image
    result, err := scanner.ScanImage(context.Background(), "nginx:latest", "HIGH,CRITICAL")
    if err != nil {
        log.Fatal(err)
    }

    // Display results
    fmt.Println(scanner.FormatScanSummary(result))
}
```

#### 2. Unified Scanner Usage

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

    // Scan image with both scanners
    result, err := scanner.ScanImage(context.Background(), "nginx:latest", "HIGH,CRITICAL")
    if err != nil {
        log.Fatal(err)
    }

    // Display unified results
    fmt.Println(scanner.FormatUnifiedScanSummary(result))
    fmt.Printf("Scanner agreement rate: %.1f%%\n", result.ComparisonMetrics.AgreementRate)
}
```

#### 3. Policy Enforcement

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/rs/zerolog"
    "github.com/Azure/container-kit/pkg/core/docker"
    "github.com/Azure/container-kit/pkg/core/security"
)

func main() {
    logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

    // Create policy engine
    policyEngine := security.NewPolicyEngine(logger)
    err := policyEngine.LoadDefaultPolicies()
    if err != nil {
        log.Fatal(err)
    }

    // Scan image
    scanner := docker.NewUnifiedSecurityScanner(logger)
    scanResult, err := scanner.ScanImage(context.Background(), "nginx:latest", "")
    if err != nil {
        log.Fatal(err)
    }

    // Create scan context for policy evaluation
    scanCtx := &security.SecurityScanContext{
        ImageRef:        scanResult.ImageRef,
        ScanTime:        scanResult.ScanTime,
        Vulnerabilities: scanResult.UniqueVulns,
        VulnSummary:     scanResult.CombinedSummary,
    }

    // Evaluate policies
    results, err := policyEngine.EvaluatePolicies(context.Background(), scanCtx)
    if err != nil {
        log.Fatal(err)
    }

    // Check if deployment should be blocked
    if policyEngine.ShouldBlock(results) {
        fmt.Println("❌ Deployment blocked due to policy violations")
        for _, result := range results {
            if !result.Passed {
                fmt.Printf("Policy %s failed with %d violations\n", result.PolicyName, len(result.Violations))
            }
        }
    } else {
        fmt.Println("✅ Deployment approved")
    }

    // Get violations summary
    summary := policyEngine.GetViolationsSummary(results)
    fmt.Printf("Policy Summary: %d passed, %d failed, %d blocking\n",
        summary["passed_policies"], summary["failed_policies"], summary["blocking_policies"])
}
```

#### 4. Secret Detection

```go
package main

import (
    "fmt"
    "log"

    "github.com/rs/zerolog"
    "github.com/Azure/container-kit/pkg/core/security"
)

func main() {
    logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

    // Create secret discovery
    discovery := security.NewSecretDiscovery(logger)

    // Scan directory for secrets
    results, err := discovery.ScanDirectory("/path/to/project", nil)
    if err != nil {
        log.Fatal(err)
    }

    // Display results
    summary := discovery.GenerateSummary(results)
    fmt.Printf("Found %d secrets (%d false positives)\n",
        summary.TotalFindings, summary.FalsePositives)

    for _, finding := range results {
        if !finding.FalsePositive {
            fmt.Printf("Secret found: %s in %s:%d\n",
                finding.SecretType, finding.File, finding.Line)
        }
    }
}
```

## Integration Patterns

### 1. CI/CD Pipeline Integration

#### GitHub Actions Example

```yaml
name: Security Scan
on: [push, pull_request]

jobs:
  security-scan:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3

    - name: Build Docker image
      run: docker build -t ${{ github.repository }}:${{ github.sha }} .

    - name: Install scanners
      run: |
        # Install Trivy
        curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
        # Install Grype
        curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin

    - name: Run security scan
      run: |
        go run ./cmd/security-scanner scan \
          --image ${{ github.repository }}:${{ github.sha }} \
          --severity HIGH,CRITICAL \
          --policy-file .security-policies.yaml \
          --output-format json \
          --output-file scan-results.json

    - name: Upload scan results
      uses: actions/upload-artifact@v3
      with:
        name: security-scan-results
        path: scan-results.json

    - name: Check scan results
      run: |
        if grep -q '"blocked": true' scan-results.json; then
          echo "Security scan failed - deployment blocked"
          exit 1
        fi
```

### 2. Kubernetes Integration

#### Security Admission Controller

```go
// Example admission controller webhook
func (w *SecurityWebhook) ValidateImage(ctx context.Context, image string) error {
    // Scan image
    scanner := docker.NewUnifiedSecurityScanner(w.logger)
    result, err := scanner.ScanImage(ctx, image, "HIGH,CRITICAL")
    if err != nil {
        return fmt.Errorf("scan failed: %w", err)
    }

    // Evaluate policies
    scanCtx := &security.SecurityScanContext{
        ImageRef:        image,
        ScanTime:        time.Now(),
        Vulnerabilities: result.UniqueVulns,
        VulnSummary:     result.CombinedSummary,
    }

    policyResults, err := w.policyEngine.EvaluatePolicies(ctx, scanCtx)
    if err != nil {
        return fmt.Errorf("policy evaluation failed: %w", err)
    }

    if w.policyEngine.ShouldBlock(policyResults) {
        return fmt.Errorf("image blocked by security policies")
    }

    return nil
}
```

### 3. Registry Integration

```go
// Example registry webhook
func (r *RegistryWebhook) OnImagePush(ctx context.Context, event *RegistryEvent) error {
    imageRef := fmt.Sprintf("%s/%s:%s", event.Registry, event.Repository, event.Tag)

    // Perform security scan
    scanner := docker.NewUnifiedSecurityScanner(r.logger)
    result, err := scanner.ScanImage(ctx, imageRef, "")
    if err != nil {
        r.logger.Error().Err(err).Str("image", imageRef).Msg("Security scan failed")
        return err
    }

    // Store scan results in metadata
    metadata := map[string]interface{}{
        "security_scan_timestamp": result.ScanTime,
        "vulnerability_count":     result.CombinedSummary.Total,
        "critical_vulnerabilities": result.CombinedSummary.Critical,
        "high_vulnerabilities":    result.CombinedSummary.High,
        "scanner_agreement_rate":  result.ComparisonMetrics.AgreementRate,
    }

    return r.registryClient.UpdateImageMetadata(imageRef, metadata)
}
```

## Configuration

### Security Policies

Create custom security policies in YAML format:

```yaml
# .security-policies.yaml
policies:
  - id: "strict-critical-vulns"
    name: "Strict Critical Vulnerability Policy"
    description: "Block any images with critical vulnerabilities"
    enabled: true
    severity: "critical"
    category: "vulnerability"
    rules:
      - id: "no-critical"
        type: "vulnerability_count"
        field: "critical"
        operator: "greater_than"
        value: 0
        description: "No critical vulnerabilities allowed"
    actions:
      - type: "block"
        description: "Block deployment due to critical vulnerabilities"
      - type: "notify"
        parameters:
          channel: "security-alerts"
          priority: "urgent"
        description: "Alert security team"

  - id: "cvss-threshold"
    name: "CVSS Score Threshold"
    description: "Block images with CVSS scores above 8.0"
    enabled: true
    severity: "high"
    category: "vulnerability"
    rules:
      - id: "cvss-limit"
        type: "cvss_score"
        field: "max_cvss_score"
        operator: "greater_than"
        value: 8.0
        description: "Block images with CVSS score > 8.0"
    actions:
      - type: "block"
        description: "Block due to high CVSS score"
```

### Environment Variables

```bash
# CVE Database Configuration
export NIST_API_KEY="your-nist-api-key"           # Optional but recommended
export CVE_CACHE_TTL="24h"                        # Cache time-to-live
export CVE_CACHE_SIZE="10000"                     # Max cache entries

# Scanner Configuration
export TRIVY_DB_REPOSITORY="ghcr.io/aquasecurity/trivy-db"
export GRYPE_DB_AUTO_UPDATE="true"

# Policy Engine Configuration
export SECURITY_POLICIES_FILE="/etc/security/policies.yaml"
export POLICY_ENFORCEMENT_MODE="strict"           # strict, permissive, monitor

# Logging Configuration
export LOG_LEVEL="info"                           # debug, info, warn, error
export LOG_FORMAT="json"                          # json, console
```

## Best Practices

### 1. Scanner Configuration
- Use both Trivy and Grype for comprehensive coverage
- Set appropriate severity thresholds for your environment
- Regularly update scanner databases
- Monitor scanner agreement rates

### 2. Policy Management
- Start with default policies and customize as needed
- Use different policies for different environments (dev, staging, prod)
- Implement policy-as-code with version control
- Test policy changes in non-production environments first

### 3. Secret Detection
- Configure exclusion patterns for your specific codebase
- Regularly review and update false positive patterns
- Implement secret verification where possible
- Integrate with secret management systems

### 4. Performance Optimization
- Use caching for CVE database lookups
- Implement parallel scanning where possible
- Set appropriate timeouts for scanner operations
- Monitor scan performance and adjust accordingly

### 5. Security Operations
- Implement automated alerting for policy violations
- Create dashboards for security metrics
- Establish incident response procedures
- Regularly review and update security policies

## Troubleshooting

### Common Issues

#### Scanner Not Found
```
Error: trivy not available: trivy executable not found in PATH
```
**Solution**: Install the required scanner or ensure it's in your PATH.

#### Policy Evaluation Errors
```
Error: invalid value type for vulnerability count rule
```
**Solution**: Ensure policy rule values are properly typed (use float64 for numeric values).

#### CVE Database Connection Issues
```
Error: failed to fetch CVE data: context deadline exceeded
```
**Solution**: Check network connectivity and consider increasing timeout values.

#### High Memory Usage
```
Scanner consuming excessive memory during scan
```
**Solution**: Implement scanner resource limits and consider scanning smaller image layers.

### Debug Mode

Enable debug logging for detailed troubleshooting:

```go
logger := zerolog.New(zerolog.NewConsoleWriter()).
    Level(zerolog.DebugLevel).
    With().
    Timestamp().
    Logger()
```

### Health Checks

Monitor scanner health:

```go
// Check scanner availability
trivyAvailable := scanner.CheckTrivyInstalled()
grypeAvailable := scanner.CheckGrypeInstalled()

if !trivyAvailable && !grypeAvailable {
    log.Fatal("No vulnerability scanners available")
}
```

## API Reference

For detailed API documentation, see:
- [Trivy Scanner API](../pkg/core/docker/trivy.go)
- [Grype Scanner API](../pkg/core/docker/grype.go)
- [Unified Scanner API](../pkg/core/docker/unified_scanner.go)
- [Policy Engine API](../pkg/core/security/policy_engine.go)
- [Secret Discovery API](../pkg/core/security/secret_discovery.go)
- [CVE Database API](../pkg/core/security/cve_database.go)

## Contributing

To contribute to the security scanning framework:

1. Follow the existing code patterns and conventions
2. Add comprehensive tests for new features
3. Update documentation for any API changes
4. Ensure all security tests pass
5. Consider performance implications of changes

## Security Considerations

- Always validate input parameters
- Implement proper error handling
- Use secure defaults for all configurations
- Regularly update scanner databases and dependencies
- Monitor for new vulnerability patterns and update accordingly
- Follow the principle of least privilege for service accounts
