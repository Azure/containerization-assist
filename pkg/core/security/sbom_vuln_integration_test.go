package security

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSBOMVulnerabilityIntegrator(t *testing.T) {
	logger := zerolog.Nop()
	sbomGenerator := NewSBOMGenerator(logger)
	cveDB := NewCVEDatabase(logger, "")
	policyEngine := NewPolicyEngine(logger)
	metricsCollector := NewMetricsCollector(logger, "test")

	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		sbomGenerator,
		cveDB,
		policyEngine,
		metricsCollector,
	)

	assert.NotNil(t, integrator)
	assert.Equal(t, sbomGenerator, integrator.sbomGenerator)
	assert.Equal(t, cveDB, integrator.cveDatabase)
	assert.Equal(t, policyEngine, integrator.policyEngine)
	assert.Equal(t, metricsCollector, integrator.metricsCollector)
}

func TestSBOMVulnerabilityIntegrator_ParsePURL(t *testing.T) {
	tests := []struct {
		name     string
		purl     string
		expected *PURLInfo
	}{
		{
			name: "npm package",
			purl: "pkg:npm/express@4.18.2",
			expected: &PURLInfo{
				Type:    "npm",
				Name:    "express",
				Version: "4.18.2",
			},
		},
		{
			name: "go package with namespace",
			purl: "pkg:golang/github.com/rs/zerolog@v1.29.0",
			expected: &PURLInfo{
				Type:      "golang",
				Namespace: "github.com",
				Name:      "rs/zerolog",
				Version:   "v1.29.0",
			},
		},
		{
			name: "pypi package",
			purl: "pkg:pypi/requests@2.28.2",
			expected: &PURLInfo{
				Type:    "pypi",
				Name:    "requests",
				Version: "2.28.2",
			},
		},
		{
			name: "package without version",
			purl: "pkg:npm/express",
			expected: &PURLInfo{
				Type: "npm",
				Name: "express",
			},
		},
		{
			name:     "invalid purl",
			purl:     "invalid-purl",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePURL(tt.purl)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSBOMVulnerabilityIntegrator_ExtractFromSPDX(t *testing.T) {
	logger := zerolog.Nop()
	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		NewSBOMGenerator(logger),
		nil, nil, nil,
	)

	spdx := &SPDXDocument{
		Packages: []SPDXPackage{
			{
				SPDXID:          "SPDXRef-Package-0",
				Name:            "express",
				Version:         "4.18.2",
				LicenseDeclared: "MIT",
				ExternalRefs: []SPDXExternalRef{
					{
						Category: "PACKAGE-MANAGER",
						Type:     "purl",
						Locator:  "pkg:npm/express@4.18.2",
					},
					{
						Category: "SECURITY",
						Type:     "cpe23Type",
						Locator:  "cpe:2.3:a:expressjs:express:4.18.2:*:*:*:*:*:*:*",
					},
				},
				Checksums: []SPDXChecksum{
					{
						Algorithm: "SHA256",
						Value:     "abc123",
					},
				},
			},
		},
	}

	packages := integrator.extractFromSPDX(spdx)

	assert.Len(t, packages, 1)
	pkg := packages[0]
	assert.Equal(t, "express", pkg.Name)
	assert.Equal(t, "4.18.2", pkg.Version)
	assert.Equal(t, "npm", pkg.Type)
	assert.Equal(t, "MIT", pkg.License)
	assert.Equal(t, "pkg:npm/express@4.18.2", pkg.PURL)
	assert.Contains(t, pkg.CPE, "express")
	assert.Equal(t, "abc123", pkg.Checksums["SHA256"])
}

func TestSBOMVulnerabilityIntegrator_ExtractFromCycloneDX(t *testing.T) {
	logger := zerolog.Nop()
	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		NewSBOMGenerator(logger),
		nil, nil, nil,
	)

	cyclonedx := &CycloneDXBOM{
		Components: []CycloneDXComponent{
			{
				Type:    "library",
				Name:    "flask",
				Version: "2.2.3",
				PURL:    "pkg:pypi/flask@2.2.3",
				CPE:     "cpe:2.3:a:pallets:flask:2.2.3:*:*:*:*:*:*:*",
				Licenses: []CycloneDXLicense{
					{
						License: &CycloneDXLicenseChoice{
							ID: "BSD-3-Clause",
						},
					},
				},
				Hashes: []CycloneDXHash{
					{
						Algorithm: "SHA-256",
						Content:   "def456",
					},
				},
				Properties: []CycloneDXProperty{
					{
						Name:  "supplier",
						Value: "Pallets",
					},
				},
			},
		},
	}

	packages := integrator.extractFromCycloneDX(cyclonedx)

	assert.Len(t, packages, 1)
	pkg := packages[0]
	assert.Equal(t, "flask", pkg.Name)
	assert.Equal(t, "2.2.3", pkg.Version)
	assert.Equal(t, "library", pkg.Type)
	assert.Equal(t, "BSD-3-Clause", pkg.License)
	assert.Equal(t, "pkg:pypi/flask@2.2.3", pkg.PURL)
	assert.Contains(t, pkg.CPE, "flask")
	assert.Equal(t, "def456", pkg.Checksums["SHA-256"])
	assert.Equal(t, "Pallets", pkg.Supplier)
}

func TestSBOMVulnerabilityIntegrator_CalculateRiskScore(t *testing.T) {
	logger := zerolog.Nop()
	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		NewSBOMGenerator(logger),
		nil, nil, nil,
	)

	t.Run("no vulnerabilities", func(t *testing.T) {
		score := integrator.calculateRiskScore([]Vulnerability{})
		assert.Equal(t, 0.0, score)
	})

	t.Run("single vulnerability", func(t *testing.T) {
		vulns := []Vulnerability{
			{
				CVSS: CVSSInfo{Score: 8.5},
			},
		}
		score := integrator.calculateRiskScore(vulns)
		assert.Equal(t, 8.5, score) // Max score * 0.7 + avg score * 0.3 = 8.5 * 0.7 + 8.5 * 0.3 = 8.5
	})

	t.Run("multiple vulnerabilities", func(t *testing.T) {
		vulns := []Vulnerability{
			{CVSS: CVSSInfo{Score: 9.0}},
			{CVSS: CVSSInfo{Score: 7.0}},
			{CVSS: CVSSInfo{Score: 5.0}},
		}
		score := integrator.calculateRiskScore(vulns)
		// Max: 9.0, Avg: 7.0, Expected: 9.0 * 0.7 + 7.0 * 0.3 = 6.3 + 2.1 = 8.4
		assert.InDelta(t, 8.4, score, 0.01)
	})
}

func TestSBOMVulnerabilityIntegrator_GenerateRecommendations(t *testing.T) {
	logger := zerolog.Nop()
	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		NewSBOMGenerator(logger),
		nil, nil, nil,
	)

	pkg := Package{
		Name:    "express",
		Version: "4.18.2",
	}

	vulns := []Vulnerability{
		{
			VulnerabilityID: "CVE-2023-1234",
			Severity:        "CRITICAL",
		},
		{
			VulnerabilityID: "CVE-2023-5678",
			Severity:        "HIGH",
		},
		{
			VulnerabilityID: "CVE-2023-9999",
			Severity:        "MEDIUM",
		},
	}

	recommendations := integrator.generateRecommendations(pkg, vulns)

	assert.GreaterOrEqual(t, len(recommendations), 4)
	assert.Contains(t, recommendations[0], "Update express to the latest version")

	// Check for severity-specific recommendations
	found := make(map[string]bool)
	for _, rec := range recommendations {
		if strings.Contains(rec, "URGENT") {
			found["critical"] = true
		}
		if strings.Contains(rec, "HIGH PRIORITY") {
			found["high"] = true
		}
		if strings.Contains(rec, "Consider patching") {
			found["medium"] = true
		}
	}
	assert.True(t, found["critical"])
	assert.True(t, found["high"])
	assert.True(t, found["medium"])
}

func TestSBOMVulnerabilityIntegrator_CalculateVulnerabilitySummary(t *testing.T) {
	logger := zerolog.Nop()
	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		NewSBOMGenerator(logger),
		nil, nil, nil,
	)

	vulns := []Vulnerability{
		{Severity: "CRITICAL", FixedVersion: "1.1.0"},
		{Severity: "CRITICAL"},
		{Severity: "HIGH", FixedVersion: "1.1.0"},
		{Severity: "MEDIUM"},
		{Severity: "LOW"},
	}

	summary := integrator.calculateVulnerabilitySummary(vulns)

	assert.Equal(t, 5, summary.Total)
	assert.Equal(t, 2, summary.Critical)
	assert.Equal(t, 1, summary.High)
	assert.Equal(t, 1, summary.Medium)
	assert.Equal(t, 1, summary.Low)
	assert.Equal(t, 2, summary.Fixable)
}

func TestSBOMVulnerabilityIntegrator_ScanWithSBOM(t *testing.T) {
	logger := zerolog.Nop()
	sbomGenerator := NewSBOMGenerator(logger)
	policyEngine := NewPolicyEngine(logger)
	err := policyEngine.LoadDefaultPolicies()
	require.NoError(t, err)

	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		sbomGenerator,
		nil, // No CVE DB for this test
		policyEngine,
		nil, // No metrics collector for this test
	)

	// Create temporary directory with test files
	tmpDir := t.TempDir()

	// Create a package.json with a potentially vulnerable package
	packageJSON := `{
		"name": "test-vulnerable-app",
		"version": "1.0.0",
		"license": "MIT",
		"dependencies": {
			"vulnerable": "1.0.0",
			"safe-package": "2.0.0"
		}
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0600)
	require.NoError(t, err)

	// Test SPDX format
	t.Run("SPDX format", func(t *testing.T) {
		ctx := context.Background()
		options := SBOMScanOptions{
			IncludeCVEEnrichment: false,
			SeverityThreshold:    "MEDIUM",
		}

		result, err := integrator.ScanWithSBOM(ctx, tmpDir, SBOMFormatSPDX, options)
		require.NoError(t, err)

		assert.NotNil(t, result)
		assert.Equal(t, tmpDir, result.Source)
		assert.Equal(t, SBOMFormatSPDX, result.Format)
		assert.NotZero(t, result.ScanDuration)

		// Verify SBOM was generated
		spdx, ok := result.SBOM.(*SPDXDocument)
		assert.True(t, ok)
		assert.NotNil(t, spdx)
		assert.GreaterOrEqual(t, len(spdx.Packages), 1)

		// Verify vulnerability report
		assert.GreaterOrEqual(t, result.VulnerabilityReport.Summary.Total, 0)
		assert.GreaterOrEqual(t, result.Metrics.TotalComponents, 1)

		// Check if any vulnerable components were detected
		// Our test package.json has "vulnerable": "1.0.0" which should trigger our detection logic
		assert.GreaterOrEqual(t, len(result.VulnerabilityReport.ComponentVulnerabilities), 1,
			"Should detect at least one vulnerable component")

		// Verify that each vulnerable component has vulnerabilities and recommendations
		for _, comp := range result.VulnerabilityReport.ComponentVulnerabilities {
			assert.GreaterOrEqual(t, len(comp.Vulnerabilities), 1,
				"Vulnerable component should have vulnerabilities")
			assert.GreaterOrEqual(t, len(comp.Recommendations), 1,
				"Vulnerable component should have recommendations")
		}
	})

	// Test CycloneDX format
	t.Run("CycloneDX format", func(t *testing.T) {
		ctx := context.Background()
		options := SBOMScanOptions{
			IncludeCVEEnrichment: false,
			SeverityThreshold:    "MEDIUM",
		}

		result, err := integrator.ScanWithSBOM(ctx, tmpDir, SBOMFormatCycloneDX, options)
		require.NoError(t, err)

		assert.NotNil(t, result)
		assert.Equal(t, SBOMFormatCycloneDX, result.Format)

		// Verify SBOM was generated
		cyclonedx, ok := result.SBOM.(*CycloneDXBOM)
		assert.True(t, ok)
		assert.NotNil(t, cyclonedx)
		assert.GreaterOrEqual(t, len(cyclonedx.Components), 1)
	})
}

func TestSBOMVulnerabilityIntegrator_ShouldSkipPackage(t *testing.T) {
	logger := zerolog.Nop()
	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		NewSBOMGenerator(logger),
		nil, nil, nil,
	)

	tests := []struct {
		name     string
		pkg      Package
		options  SBOMScanOptions
		expected bool
	}{
		{
			name: "should not skip normal package",
			pkg:  Package{Type: "npm"},
			options: SBOMScanOptions{
				ExcludeTypes: []string{"test"},
			},
			expected: false,
		},
		{
			name: "should skip excluded type",
			pkg:  Package{Type: "test"},
			options: SBOMScanOptions{
				ExcludeTypes: []string{"test", "dev"},
			},
			expected: true,
		},
		{
			name: "should skip if type in exclude list",
			pkg:  Package{Type: "dev"},
			options: SBOMScanOptions{
				ExcludeTypes: []string{"test", "dev"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := integrator.shouldSkipPackage(tt.pkg, tt.options)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSBOMVulnerabilityIntegrator_WriteEnrichedResult(t *testing.T) {
	logger := zerolog.Nop()
	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		NewSBOMGenerator(logger),
		nil, nil, nil,
	)

	result := &EnrichedSBOMResult{
		Source:      "test-source",
		Format:      SBOMFormatSPDX,
		GeneratedAt: time.Now(),
		SBOM: &SPDXDocument{
			SPDXVersion: "SPDX-2.3",
			Name:        "Test SBOM",
		},
		VulnerabilityReport: VulnerabilityReport{
			Summary: VulnerabilitySummary{
				Total:    5,
				Critical: 1,
				High:     2,
			},
		},
		Metrics: SBOMScanMetrics{
			TotalComponents: 10,
			CVEsFound:       5,
		},
	}

	// Write to temporary file
	tmpFile := filepath.Join(t.TempDir(), "enriched-sbom.json")
	err := integrator.WriteEnrichedResult(result, tmpFile)
	require.NoError(t, err)

	// Verify file was created and can be read
	_, err = os.Stat(tmpFile)
	assert.NoError(t, err)

	// Verify content is valid JSON
	content, err := os.ReadFile(tmpFile)
	require.NoError(t, err)

	var decoded EnrichedSBOMResult
	err = json.Unmarshal(content, &decoded)
	require.NoError(t, err)

	assert.Equal(t, result.Source, decoded.Source)
	assert.Equal(t, result.Format, decoded.Format)
	assert.Equal(t, result.VulnerabilityReport.Summary.Total, decoded.VulnerabilityReport.Summary.Total)
}

func TestSBOMVulnerabilityIntegrator_MapCycloneDXTypeToPackageType(t *testing.T) {
	logger := zerolog.Nop()
	integrator := NewSBOMVulnerabilityIntegrator(
		logger,
		NewSBOMGenerator(logger),
		nil, nil, nil,
	)

	tests := []struct {
		cycloneDXType string
		expected      string
	}{
		{"library", "library"},
		{"container", "container"},
		{"operating-system", "os"},
		{"application", "application"},
		{"unknown-type", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.cycloneDXType, func(t *testing.T) {
			result := integrator.mapCycloneDXTypeToPackageType(tt.cycloneDXType)
			assert.Equal(t, tt.expected, result)
		})
	}
}
