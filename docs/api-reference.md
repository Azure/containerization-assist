# Security Scanning Framework - API Reference

This document provides detailed API reference for all components of the security scanning framework.

## Package Structure

```
pkg/core/
├── docker/                    # Container scanning components
│   ├── trivy.go              # Trivy vulnerability scanner
│   ├── trivy_test.go         # Trivy scanner tests
│   ├── grype.go              # Grype vulnerability scanner
│   ├── unified_scanner.go    # Unified multi-scanner
│   └── registry.go           # Docker registry health checks
├── security/                 # Security analysis components
│   ├── vulnerability.go      # Vulnerability data structures
│   ├── cve_database.go       # NIST CVE database integration
│   ├── cve_database_test.go  # CVE database tests
│   ├── secret_discovery.go   # Secret detection engine
│   ├── secret_discovery_test.go # Secret detection tests
│   ├── policy_engine.go      # Security policy enforcement
│   └── policy_engine_test.go # Policy engine tests
└── mcp/                      # MCP tool implementations
    ├── container_scanner.go  # MCP container scanning tool
    └── secret_scanner.go     # MCP secret scanning tool
```

## Core Data Structures

### Vulnerability

```go
type Vulnerability struct {
    VulnerabilityID    string                 `json:"vulnerability_id"`
    PkgName           string                 `json:"pkg_name"`
    PkgID             string                 `json:"pkg_id,omitempty"`
    PkgPath           string                 `json:"pkg_path,omitempty"`
    PkgType           string                 `json:"pkg_type,omitempty"`
    InstalledVersion  string                 `json:"installed_version"`
    FixedVersion      string                 `json:"fixed_version,omitempty"`
    Severity          string                 `json:"severity"`
    Title             string                 `json:"title,omitempty"`
    Description       string                 `json:"description,omitempty"`
    References        []string               `json:"references,omitempty"`
    PublishedDate     string                 `json:"published_date,omitempty"`
    LastModifiedDate  string                 `json:"last_modified_date,omitempty"`
    CVSS              CVSSInfo               `json:"cvss,omitempty"`
    CVSSV3            CVSSV3Info             `json:"cvss_v3,omitempty"`
    CWE               []string               `json:"cwe,omitempty"`
    DataSource        VulnDataSource         `json:"data_source,omitempty"`
    VendorSeverity    map[string]string      `json:"vendor_severity,omitempty"`
    PkgIdentifier     PkgIdentifier          `json:"pkg_identifier,omitempty"`
    Layer             string                 `json:"layer,omitempty"`
    Status            string                 `json:"status,omitempty"`
    PrimaryURL        string                 `json:"primary_url,omitempty"`
}
```

### VulnerabilitySummary

```go
type VulnerabilitySummary struct {
    Total    int `json:"total"`
    Critical int `json:"critical"`
    High     int `json:"high"`
    Medium   int `json:"medium"`
    Low      int `json:"low"`
    Unknown  int `json:"unknown"`
    Fixable  int `json:"fixable"`
}
```

### SecretFinding

```go
type SecretFinding struct {
    File           string            `json:"file"`
    Line           int               `json:"line"`
    Column         int               `json:"column,omitempty"`
    SecretType     string            `json:"secret_type"`
    Match          string            `json:"match"`
    Context        string            `json:"context,omitempty"`
    Entropy        float64           `json:"entropy,omitempty"`
    FalsePositive  bool              `json:"false_positive"`
    Verified       bool              `json:"verified"`
    Confidence     ConfidenceLevel   `json:"confidence"`
    Metadata       map[string]string `json:"metadata,omitempty"`
}
```

## Scanner APIs

### TrivyScanner

#### Constructor
```go
func NewTrivyScanner(logger zerolog.Logger) *TrivyScanner
```
Creates a new Trivy scanner instance.

#### Methods

##### ScanImage
```go
func (ts *TrivyScanner) ScanImage(ctx context.Context, imageRef string, severityThreshold string) (*ScanResult, error)
```
Scans a Docker image for vulnerabilities using Trivy.

**Parameters:**
- `ctx`: Context for the operation
- `imageRef`: Docker image reference (e.g., "nginx:latest")
- `severityThreshold`: Comma-separated severity levels (e.g., "HIGH,CRITICAL")

**Returns:**
- `*ScanResult`: Detailed scan results
- `error`: Error if scan fails

##### CheckTrivyInstalled
```go
func (ts *TrivyScanner) CheckTrivyInstalled() bool
```
Checks if Trivy is available on the system.

##### FormatScanSummary
```go
func (ts *TrivyScanner) FormatScanSummary(result *ScanResult) string
```
Formats scan results for human-readable display.

### GrypeScanner

#### Constructor
```go
func NewGrypeScanner(logger zerolog.Logger) *GrypeScanner
```
Creates a new Grype scanner instance.

#### Methods

##### ScanImage
```go
func (gs *GrypeScanner) ScanImage(ctx context.Context, imageRef string, severityThreshold string) (*ScanResult, error)
```
Scans a Docker image for vulnerabilities using Grype.

##### CheckGrypeInstalled
```go
func (gs *GrypeScanner) CheckGrypeInstalled() bool
```
Checks if Grype is available on the system.

##### InstallGrype
```go
func (gs *GrypeScanner) InstallGrype() string
```
Returns installation instructions for Grype.

### UnifiedSecurityScanner

#### Constructor
```go
func NewUnifiedSecurityScanner(logger zerolog.Logger) *UnifiedSecurityScanner
```
Creates a unified scanner that combines Trivy and Grype.

#### Methods

##### ScanImage
```go
func (us *UnifiedSecurityScanner) ScanImage(ctx context.Context, imageRef string, severityThreshold string) (*UnifiedScanResult, error)
```
Performs comprehensive security scan using all available scanners.

**Returns:**
- `*UnifiedScanResult`: Combined results from multiple scanners

##### GetAvailableScanners
```go
func (us *UnifiedSecurityScanner) GetAvailableScanners() map[string]bool
```
Returns which scanners are available on the system.

##### FormatUnifiedScanSummary
```go
func (us *UnifiedSecurityScanner) FormatUnifiedScanSummary(result *UnifiedScanResult) string
```
Formats unified scan results for display.

## Security Policy Engine

### PolicyEngine

#### Constructor
```go
func NewPolicyEngine(logger zerolog.Logger) *PolicyEngine
```
Creates a new security policy engine.

#### Methods

##### LoadDefaultPolicies
```go
func (pe *PolicyEngine) LoadDefaultPolicies() error
```
Loads a set of default security policies.

##### LoadPolicies
```go
func (pe *PolicyEngine) LoadPolicies(policies []SecurityPolicy) error
```
Loads custom security policies.

##### EvaluatePolicies
```go
func (pe *PolicyEngine) EvaluatePolicies(ctx context.Context, scanCtx *SecurityScanContext) ([]PolicyEvaluationResult, error)
```
Evaluates all enabled policies against scan context.

**Parameters:**
- `ctx`: Context for the operation
- `scanCtx`: Security scan context containing vulnerability data

**Returns:**
- `[]PolicyEvaluationResult`: Results of policy evaluation
- `error`: Error if evaluation fails

##### ShouldBlock
```go
func (pe *PolicyEngine) ShouldBlock(results []PolicyEvaluationResult) bool
```
Determines if any policy violations should block deployment.

##### GetViolationsSummary
```go
func (pe *PolicyEngine) GetViolationsSummary(results []PolicyEvaluationResult) map[string]interface{}
```
Returns a summary of all policy violations.

##### Policy Management
```go
func (pe *PolicyEngine) AddPolicy(policy SecurityPolicy) error
func (pe *PolicyEngine) UpdatePolicy(policy SecurityPolicy) error
func (pe *PolicyEngine) RemovePolicy(id string) error
func (pe *PolicyEngine) GetPolicyByID(id string) (*SecurityPolicy, error)
func (pe *PolicyEngine) GetPolicies() []SecurityPolicy
```
Methods for managing security policies.

### Policy Rule Types

#### RuleType Constants
```go
const (
    RuleTypeVulnerabilityCount    RuleType = "vulnerability_count"
    RuleTypeVulnerabilitySeverity RuleType = "vulnerability_severity"
    RuleTypeCVSSScore             RuleType = "cvss_score"
    RuleTypeSecretPresence        RuleType = "secret_presence"
    RuleTypePackageVersion        RuleType = "package_version"
    RuleTypeImageAge              RuleType = "image_age"
    RuleTypeImageSize             RuleType = "image_size"
    RuleTypeLicense               RuleType = "license"
    RuleTypeCompliance            RuleType = "compliance"
)
```

#### RuleOperator Constants
```go
const (
    OperatorEquals              RuleOperator = "equals"
    OperatorNotEquals           RuleOperator = "not_equals"
    OperatorGreaterThan         RuleOperator = "greater_than"
    OperatorGreaterThanOrEqual  RuleOperator = "greater_than_or_equal"
    OperatorLessThan            RuleOperator = "less_than"
    OperatorLessThanOrEqual     RuleOperator = "less_than_or_equal"
    OperatorContains            RuleOperator = "contains"
    OperatorNotContains         RuleOperator = "not_contains"
    OperatorMatches             RuleOperator = "matches"
    OperatorNotMatches          RuleOperator = "not_matches"
    OperatorIn                  RuleOperator = "in"
    OperatorNotIn               RuleOperator = "not_in"
)
```

## Secret Discovery

### SecretDiscovery

#### Constructor
```go
func NewSecretDiscovery(logger zerolog.Logger) *SecretDiscovery
```
Creates a new secret discovery engine.

#### Methods

##### ScanDirectory
```go
func (sd *SecretDiscovery) ScanDirectory(rootPath string, exclusionManager *ExclusionManager) ([]SecretFinding, error)
```
Scans a directory tree for exposed secrets.

**Parameters:**
- `rootPath`: Root directory to scan
- `exclusionManager`: Optional exclusion manager for filtering files

**Returns:**
- `[]SecretFinding`: List of detected secrets
- `error`: Error if scan fails

##### ScanFile
```go
func (sd *SecretDiscovery) ScanFile(filePath string) ([]SecretFinding, error)
```
Scans a single file for secrets.

##### ScanContent
```go
func (sd *SecretDiscovery) ScanContent(content, fileName string) []SecretFinding
```
Scans string content for secrets.

##### GenerateSummary
```go
func (sd *SecretDiscovery) GenerateSummary(findings []SecretFinding) *DiscoverySummary
```
Generates a summary of secret discovery results.

##### VerifySecret
```go
func (sd *SecretDiscovery) VerifySecret(finding *SecretFinding) error
```
Attempts to verify if a detected secret is valid.

### Secret Pattern Constants
```go
const (
    SecretTypeAWSAccessKey   = "aws_access_key"
    SecretTypeAWSSecretKey   = "aws_secret_key"
    SecretTypeGitHubToken    = "github_token"
    SecretTypeJWTToken       = "jwt_token"
    SecretTypePrivateKey     = "private_key"
    SecretTypeDatabaseURL    = "database_url"
    SecretTypeAPIKey         = "api_key"
    SecretTypePassword       = "password"
    SecretTypeGenericSecret  = "generic_secret"
)
```

## CVE Database Integration

### CVEDatabase

#### Constructor
```go
func NewCVEDatabase(logger zerolog.Logger) *CVEDatabase
```
Creates a new CVE database client.

#### Methods

##### GetCVE
```go
func (db *CVEDatabase) GetCVE(ctx context.Context, cveID string) (*CVERecord, error)
```
Retrieves detailed CVE information from NIST NVD.

##### EnrichVulnerability
```go
func (db *CVEDatabase) EnrichVulnerability(ctx context.Context, vuln *Vulnerability) error
```
Enriches vulnerability data with additional CVE information.

##### SearchCVEs
```go
func (db *CVEDatabase) SearchCVEs(ctx context.Context, params SearchParams) (*SearchResponse, error)
```
Searches for CVEs based on various criteria.

##### GetCacheStats
```go
func (db *CVEDatabase) GetCacheStats() CacheStats
```
Returns cache statistics for monitoring.

##### ClearCache
```go
func (db *CVEDatabase) ClearCache()
```
Clears the CVE cache.

##### SetAPIKey
```go
func (db *CVEDatabase) SetAPIKey(apiKey string)
```
Sets the NIST API key for enhanced rate limits.

## Docker Registry Health

### RegistryHealthChecker

#### Constructor
```go
func NewRegistryHealthChecker(logger zerolog.Logger) *RegistryHealthChecker
```
Creates a new registry health checker.

#### Methods

##### CheckHealth
```go
func (rhc *RegistryHealthChecker) CheckHealth(ctx context.Context, registryURL string, auth *RegistryAuth) (*HealthStatus, error)
```
Checks the health of a Docker registry.

##### CheckImageExists
```go
func (rhc *RegistryHealthChecker) CheckImageExists(ctx context.Context, imageRef string, auth *RegistryAuth) (bool, error)
```
Checks if a specific image exists in the registry.

##### GetRegistryInfo
```go
func (rhc *RegistryHealthChecker) GetRegistryInfo(ctx context.Context, registryURL string, auth *RegistryAuth) (*RegistryInfo, error)
```
Retrieves detailed registry information.

## Error Types

### Common Errors
```go
var (
    ErrScannerNotFound     = errors.New("vulnerability scanner not found")
    ErrInvalidImageRef     = errors.New("invalid image reference")
    ErrPolicyViolation     = errors.New("security policy violation")
    ErrCVENotFound         = errors.New("CVE not found")
    ErrRegistryUnreachable = errors.New("registry unreachable")
)
```

## Configuration Types

### ScanConfig
```go
type ScanConfig struct {
    SeverityThreshold  string        `json:"severity_threshold,omitempty"`
    Timeout           time.Duration  `json:"timeout,omitempty"`
    MaxConcurrency    int           `json:"max_concurrency,omitempty"`
    EnableCache       bool          `json:"enable_cache"`
    CacheTTL          time.Duration `json:"cache_ttl,omitempty"`
}
```

### PolicyConfig
```go
type PolicyConfig struct {
    PolicyFile        string `json:"policy_file,omitempty"`
    EnforcementMode   string `json:"enforcement_mode,omitempty"` // strict, permissive, monitor
    NotificationURL   string `json:"notification_url,omitempty"`
    BlockOnFailure    bool   `json:"block_on_failure"`
}
```

## Usage Examples

### Basic Vulnerability Scanning
```go
// Create scanner
scanner := docker.NewTrivyScanner(logger)

// Scan image
result, err := scanner.ScanImage(ctx, "nginx:latest", "HIGH,CRITICAL")
if err != nil {
    return err
}

// Check results
if result.Summary.Critical > 0 {
    log.Printf("Found %d critical vulnerabilities", result.Summary.Critical)
}
```

### Policy-based Scanning
```go
// Create policy engine
engine := security.NewPolicyEngine(logger)
err := engine.LoadDefaultPolicies()
if err != nil {
    return err
}

// Scan and evaluate
scanner := docker.NewUnifiedSecurityScanner(logger)
scanResult, err := scanner.ScanImage(ctx, imageRef, "")
if err != nil {
    return err
}

// Create scan context
scanCtx := &security.SecurityScanContext{
    ImageRef:        scanResult.ImageRef,
    ScanTime:        scanResult.ScanTime,
    Vulnerabilities: scanResult.UniqueVulns,
    VulnSummary:     scanResult.CombinedSummary,
}

// Evaluate policies
results, err := engine.EvaluatePolicies(ctx, scanCtx)
if err != nil {
    return err
}

// Check if blocked
if engine.ShouldBlock(results) {
    return errors.New("deployment blocked by security policies")
}
```

### Secret Detection
```go
// Create secret discovery
discovery := security.NewSecretDiscovery(logger)

// Scan directory
findings, err := discovery.ScanDirectory("/path/to/project", nil)
if err != nil {
    return err
}

// Filter false positives
realSecrets := make([]security.SecretFinding, 0)
for _, finding := range findings {
    if !finding.FalsePositive {
        realSecrets = append(realSecrets, finding)
    }
}

if len(realSecrets) > 0 {
    return fmt.Errorf("found %d exposed secrets", len(realSecrets))
}
```

## Testing

All components include comprehensive test suites:

- **Unit Tests**: Test individual functions and methods
- **Integration Tests**: Test component interactions
- **Mock Tests**: Test with simulated external dependencies

### Running Tests
```bash
# Run all security tests
go test ./pkg/core/security -v

# Run specific test
go test ./pkg/core/security -run TestPolicyEngine -v

# Run with coverage
go test ./pkg/core/security -cover
```

## Thread Safety

All components are designed to be thread-safe:
- **Scanners**: Can be used concurrently across goroutines
- **Policy Engine**: Safe for concurrent policy evaluation
- **CVE Database**: Cache operations are protected by mutexes
- **Secret Discovery**: Stateless operations are safe for concurrent use

## Performance Considerations

- **Caching**: CVE database implements intelligent caching
- **Parallel Execution**: Unified scanner runs multiple scanners in parallel
- **Resource Limits**: Implement timeouts and concurrency limits
- **Memory Usage**: Large images may require increased memory limits

## Migration Guide

When upgrading between versions:

1. Check for breaking changes in the API
2. Update import paths if package structure changes
3. Review and update custom policies for new rule types
4. Test scanner compatibility with new versions
5. Update CI/CD pipelines if command-line interfaces change