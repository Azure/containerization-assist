package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSBOMGenerator(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	assert.NotNil(t, generator)
	assert.Equal(t, "syft", generator.scannerBinary)
}

func TestSBOMGenerator_ParsePackageJSON(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	packageJSON := `{
		"name": "test-package",
		"version": "1.0.0",
		"license": "MIT",
		"author": "Test Author",
		"dependencies": {
			"express": "^4.18.0",
			"lodash": "~4.17.21"
		}
	}`

	pkg, err := generator.parsePackageJSON([]byte(packageJSON))
	require.NoError(t, err)

	assert.Equal(t, "test-package", pkg.Name)
	assert.Equal(t, "1.0.0", pkg.Version)
	assert.Equal(t, "npm", pkg.Type)
	assert.Equal(t, "MIT", pkg.License)
	assert.Equal(t, "Test Author", pkg.Supplier)
	assert.Equal(t, "pkg:npm/test-package@1.0.0", pkg.PURL)
	assert.Len(t, pkg.Dependencies, 2)
	assert.Contains(t, pkg.Dependencies, "express@^4.18.0")
	assert.Contains(t, pkg.Dependencies, "lodash@~4.17.21")
}

func TestSBOMGenerator_ParseGoMod(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	goMod := `module github.com/example/project

go 1.19

require (
	github.com/rs/zerolog v1.29.0
	github.com/stretchr/testify v1.8.2
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
)`

	packages, err := generator.parseGoMod([]byte(goMod))
	require.NoError(t, err)

	// Should have 5 packages (main module + 4 dependencies)
	assert.Len(t, packages, 5)

	// Check main module
	mainFound := false
	for _, pkg := range packages {
		if pkg.Name == "github.com/example/project" {
			mainFound = true
			assert.Equal(t, "go", pkg.Type)
			assert.Equal(t, "pkg:golang/github.com/example/project", pkg.PURL)
		}
	}
	assert.True(t, mainFound, "Main module not found")

	// Check dependencies
	expectedDeps := map[string]string{
		"github.com/rs/zerolog":         "v1.29.0",
		"github.com/stretchr/testify":   "v1.8.2",
		"github.com/davecgh/go-spew":    "v1.1.1",
		"github.com/pmezard/go-difflib": "v1.0.0",
	}

	for _, pkg := range packages {
		if version, exists := expectedDeps[pkg.Name]; exists {
			assert.Equal(t, version, pkg.Version)
			assert.Equal(t, "go", pkg.Type)
			assert.Equal(t, "pkg:golang/"+pkg.Name+"@"+pkg.Version, pkg.PURL)
		}
	}
}

func TestSBOMGenerator_ParseRequirementsTxt(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	requirements := `# This is a comment
requests==2.28.2
flask==2.2.3
numpy==1.24.2
# Another comment
pandas==1.5.3
`

	packages, err := generator.parseRequirementsTxt([]byte(requirements))
	require.NoError(t, err)

	assert.Len(t, packages, 4)

	expectedPkgs := map[string]string{
		"requests": "2.28.2",
		"flask":    "2.2.3",
		"numpy":    "1.24.2",
		"pandas":   "1.5.3",
	}

	for _, pkg := range packages {
		expectedVersion, exists := expectedPkgs[pkg.Name]
		assert.True(t, exists)
		assert.Equal(t, expectedVersion, pkg.Version)
		assert.Equal(t, "pip", pkg.Type)
		assert.Equal(t, "pkg:pypi/"+pkg.Name+"@"+pkg.Version, pkg.PURL)
	}
}

func TestSBOMGenerator_CreateSPDXDocument(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	packages := []Package{
		{
			Name:     "test-package",
			Version:  "1.0.0",
			Type:     "npm",
			License:  "MIT",
			PURL:     "pkg:npm/test-package@1.0.0",
			CPE:      "cpe:2.3:a:test:test-package:1.0.0:*:*:*:*:*:*:*",
			Supplier: "Test Inc",
			Checksums: map[string]string{
				"SHA256": "abcd1234",
			},
		},
		{
			Name:    "another-package",
			Version: "2.0.0",
			Type:    "pip",
			License: "Apache-2.0",
			PURL:    "pkg:pypi/another-package@2.0.0",
		},
	}

	doc := generator.createSPDXDocument("test-source", packages)

	assert.Equal(t, "SPDX-2.3", doc.SPDXVersion)
	assert.Equal(t, "CC0-1.0", doc.DataLicense)
	assert.Equal(t, "SPDXRef-DOCUMENT", doc.SPDXID)
	assert.Contains(t, doc.Name, "test-source")
	assert.Contains(t, doc.DocumentNamespace, "test-source")

	// Check creation info
	assert.Contains(t, doc.CreationInfo.Creators, "Tool: container-kit-security-scanner")
	assert.Equal(t, "3.19", doc.CreationInfo.LicenseListVersion)

	// Check packages
	assert.Len(t, doc.Packages, 2)

	// Check first package
	pkg1 := doc.Packages[0]
	assert.Equal(t, "SPDXRef-Package-0", pkg1.SPDXID)
	assert.Equal(t, "test-package", pkg1.Name)
	assert.Equal(t, "1.0.0", pkg1.Version)
	assert.Equal(t, "MIT", pkg1.LicenseConcluded)
	assert.Equal(t, "MIT", pkg1.LicenseDeclared)
	assert.Equal(t, "Organization: Test Inc", pkg1.Supplier)
	assert.Len(t, pkg1.Checksums, 1)
	assert.Equal(t, "SHA256", pkg1.Checksums[0].Algorithm)
	assert.Equal(t, "abcd1234", pkg1.Checksums[0].Value)
	assert.Len(t, pkg1.ExternalRefs, 2)

	// Check external refs
	purlFound := false
	cpeFound := false
	for _, ref := range pkg1.ExternalRefs {
		if ref.Type == "purl" {
			purlFound = true
			assert.Equal(t, "PACKAGE-MANAGER", ref.Category)
			assert.Equal(t, "pkg:npm/test-package@1.0.0", ref.Locator)
		}
		if ref.Type == "cpe23Type" {
			cpeFound = true
			assert.Equal(t, "SECURITY", ref.Category)
			assert.Contains(t, ref.Locator, "test-package")
		}
	}
	assert.True(t, purlFound)
	assert.True(t, cpeFound)

	// Check relationships
	assert.Len(t, doc.Relationships, 2)
	for i, rel := range doc.Relationships {
		assert.Equal(t, "SPDXRef-DOCUMENT", rel.SPDXElementID)
		assert.Equal(t, "DESCRIBES", rel.RelationshipType)
		assert.Equal(t, fmt.Sprintf("SPDXRef-Package-%d", i), rel.RelatedSPDXElement)
	}
}

func TestSBOMGenerator_NormalizeLicense(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	tests := []struct {
		input    string
		expected string
	}{
		{"MIT", "MIT"},
		{"Apache-2.0", "Apache-2.0"},
		{"Apache 2.0", "Apache-2.0"},
		{"GPL-3.0", "GPL-3.0-only"},
		{"GPL-2.0", "GPL-2.0-only"},
		{"BSD-3-Clause", "BSD-3-Clause"},
		{"", "NOASSERTION"},
		{"CustomLicense", "CustomLicense"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := generator.normalizeLicense(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSBOMGenerator_ValidateSBOM(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	t.Run("valid SBOM", func(t *testing.T) {
		doc := &SPDXDocument{
			SPDXVersion:       "SPDX-2.3",
			DataLicense:       "CC0-1.0",
			SPDXID:            "SPDXRef-DOCUMENT",
			Name:              "Test SBOM",
			DocumentNamespace: "https://example.com/sbom/test",
			Packages: []SPDXPackage{
				{
					SPDXID:           "SPDXRef-Package-0",
					Name:             "test-package",
					DownloadLocation: "NOASSERTION",
				},
			},
			Relationships: []SPDXRelationship{
				{
					SPDXElementID:      "SPDXRef-DOCUMENT",
					RelationshipType:   "DESCRIBES",
					RelatedSPDXElement: "SPDXRef-Package-0",
				},
			},
		}

		err := generator.ValidateSBOM(doc)
		assert.NoError(t, err)
	})

	t.Run("missing SPDX version", func(t *testing.T) {
		doc := &SPDXDocument{
			DataLicense: "CC0-1.0",
			Name:        "Test",
		}
		err := generator.ValidateSBOM(doc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SPDX version is required")
	})

	t.Run("no packages", func(t *testing.T) {
		doc := &SPDXDocument{
			SPDXVersion:       "SPDX-2.3",
			DataLicense:       "CC0-1.0",
			Name:              "Test",
			DocumentNamespace: "https://example.com/test",
			Packages:          []SPDXPackage{},
		}
		err := generator.ValidateSBOM(doc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one package is required")
	})

	t.Run("invalid relationship", func(t *testing.T) {
		doc := &SPDXDocument{
			SPDXVersion:       "SPDX-2.3",
			DataLicense:       "CC0-1.0",
			SPDXID:            "SPDXRef-DOCUMENT",
			Name:              "Test",
			DocumentNamespace: "https://example.com/test",
			Packages: []SPDXPackage{
				{
					SPDXID:           "SPDXRef-Package-0",
					Name:             "test",
					DownloadLocation: "NOASSERTION",
				},
			},
			Relationships: []SPDXRelationship{
				{
					SPDXElementID:      "SPDXRef-INVALID",
					RelationshipType:   "DESCRIBES",
					RelatedSPDXElement: "SPDXRef-Package-0",
				},
			},
		}
		err := generator.ValidateSBOM(doc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SPDX element ID")
	})
}

func TestSBOMGenerator_WriteSPDX(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	doc := &SPDXDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              "Test SBOM",
		DocumentNamespace: "https://example.com/sbom/test",
		CreationInfo: SPDXCreationInfo{
			Created:  "2023-01-01T00:00:00Z",
			Creators: []string{"Tool: test"},
		},
		Packages: []SPDXPackage{
			{
				SPDXID:           "SPDXRef-Package-0",
				Name:             "test-package",
				Version:          "1.0.0",
				DownloadLocation: "NOASSERTION",
				LicenseConcluded: "MIT",
				LicenseDeclared:  "MIT",
				CopyrightText:    "NOASSERTION",
			},
		},
	}

	var buf bytes.Buffer
	err := generator.WriteSPDX(doc, &buf)
	require.NoError(t, err)

	// Verify JSON output
	var decoded SPDXDocument
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)

	assert.Equal(t, doc.SPDXVersion, decoded.SPDXVersion)
	assert.Equal(t, doc.DataLicense, decoded.DataLicense)
	assert.Equal(t, doc.Name, decoded.Name)
	assert.Len(t, decoded.Packages, 1)
	assert.Equal(t, doc.Packages[0].Name, decoded.Packages[0].Name)
}

func TestSBOMGenerator_ScanDirectory(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	// Create temporary directory with test files
	tmpDir := t.TempDir()

	// Create package.json
	packageJSON := `{
		"name": "test-app",
		"version": "1.0.0",
		"license": "MIT",
		"dependencies": {
			"express": "4.18.0"
		}
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0600)
	require.NoError(t, err)

	// Create requirements.txt
	requirements := `flask==2.2.3
requests==2.28.2`
	err = os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(requirements), 0600)
	require.NoError(t, err)

	// Create go.mod
	goMod := `module github.com/test/app

go 1.19

require (
	github.com/rs/zerolog v1.29.0
)`
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0600)
	require.NoError(t, err)

	// Scan directory
	ctx := context.Background()
	packages, err := generator.scanDirectory(ctx, tmpDir)
	require.NoError(t, err)

	// Should find packages from all three files
	assert.GreaterOrEqual(t, len(packages), 5) // At least: test-app, flask, requests, zerolog, main go module

	// Check that packages have checksums and file references
	for _, pkg := range packages {
		assert.NotEmpty(t, pkg.Checksums)
		assert.NotEmpty(t, pkg.FilesScanned)
	}
}

func TestSBOMGenerator_CalculateChecksum(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	testData := []byte("test data for checksum")
	checksum := generator.calculateChecksum(testData)

	// SHA256 should be 64 characters hex
	assert.Len(t, checksum, 64)
	assert.Regexp(t, "^[a-f0-9]{64}$", checksum)

	// Same data should produce same checksum
	checksum2 := generator.calculateChecksum(testData)
	assert.Equal(t, checksum, checksum2)

	// Different data should produce different checksum
	checksum3 := generator.calculateChecksum([]byte("different data"))
	assert.NotEqual(t, checksum, checksum3)
}

func TestSBOMGenerator_GenerateSBOM(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewSBOMGenerator(logger)

	// Create temporary directory with a simple package.json
	tmpDir := t.TempDir()
	packageJSON := `{
		"name": "test-project",
		"version": "1.0.0",
		"license": "MIT"
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0600)
	require.NoError(t, err)

	t.Run("SPDX format", func(t *testing.T) {
		// Generate SBOM
		ctx := context.Background()
		result, err := generator.GenerateSBOM(ctx, tmpDir, SBOMFormatSPDX)
		require.NoError(t, err)

		// Cast to SPDX document
		sbom, ok := result.(*SPDXDocument)
		require.True(t, ok, "Result should be an SPDX document")

		// Validate the generated SBOM
		assert.NotNil(t, sbom)
		assert.Equal(t, "SPDX-2.3", sbom.SPDXVersion)
		assert.GreaterOrEqual(t, len(sbom.Packages), 1)

		// Validate SBOM structure
		err = generator.ValidateSBOM(sbom)
		assert.NoError(t, err)
	})

	t.Run("CycloneDX format", func(t *testing.T) {
		// Generate SBOM
		ctx := context.Background()
		result, err := generator.GenerateSBOM(ctx, tmpDir, SBOMFormatCycloneDX)
		require.NoError(t, err)

		// Cast to CycloneDX BOM
		bom, ok := result.(*CycloneDXBOM)
		require.True(t, ok, "Result should be a CycloneDX BOM")

		// Validate the generated BOM
		assert.NotNil(t, bom)
		assert.Equal(t, "CycloneDX", bom.BOMFormat)
		assert.Equal(t, "1.4", bom.SpecVersion)
		assert.GreaterOrEqual(t, len(bom.Components), 1)

		// Validate BOM structure
		err = generator.ValidateCycloneDXBOM(bom)
		assert.NoError(t, err)
	})

	t.Run("unsupported format", func(t *testing.T) {
		ctx := context.Background()
		_, err := generator.GenerateSBOM(ctx, tmpDir, SBOMFormat("unsupported"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported SBOM format")
	})
}
