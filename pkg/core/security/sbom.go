// Package security provides SBOM (Software Bill of Materials) generation capabilities
package security

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// SBOMFormat represents the output format for SBOM
type SBOMFormat string

const (
	// SBOMFormatSPDX represents SPDX format
	SBOMFormatSPDX SBOMFormat = "spdx"
	// SBOMFormatCycloneDX represents CycloneDX format
	SBOMFormatCycloneDX SBOMFormat = "cyclonedx"
	// SBOMFormatSyft represents Syft native format
	SBOMFormatSyft SBOMFormat = "syft"
)

// Package represents a software package in the SBOM
type Package struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Type         string            `json:"type"` // npm, pip, gem, etc.
	PURL         string            `json:"purl"` // Package URL
	CPE          string            `json:"cpe,omitempty"`
	License      string            `json:"license,omitempty"`
	Supplier     string            `json:"supplier,omitempty"`
	FilesScanned []string          `json:"files_scanned,omitempty"`
	Checksums    map[string]string `json:"checksums,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// SPDXDocument represents an SPDX document
type SPDXDocument struct {
	SPDXVersion          string               `json:"spdxVersion"`
	DataLicense          string               `json:"dataLicense"`
	SPDXID               string               `json:"SPDXID"`
	Name                 string               `json:"name"`
	DocumentNamespace    string               `json:"documentNamespace"`
	CreationInfo         SPDXCreationInfo     `json:"creationInfo"`
	Packages             []SPDXPackage        `json:"packages"`
	Relationships        []SPDXRelationship   `json:"relationships"`
	ExternalDocumentRefs []SPDXExternalDocRef `json:"externalDocumentRefs,omitempty"`
}

// SPDXCreationInfo represents creation information in SPDX
type SPDXCreationInfo struct {
	Created            string   `json:"created"`
	Creators           []string `json:"creators"`
	LicenseListVersion string   `json:"licenseListVersion"`
}

// SPDXPackage represents a package in SPDX format
type SPDXPackage struct {
	SPDXID           string            `json:"SPDXID"`
	Name             string            `json:"name"`
	DownloadLocation string            `json:"downloadLocation"`
	FilesAnalyzed    bool              `json:"filesAnalyzed"`
	VerificationCode *SPDXVerification `json:"verificationCode,omitempty"`
	Checksums        []SPDXChecksum    `json:"checksums,omitempty"`
	HomePage         string            `json:"homePage,omitempty"`
	SourceInfo       string            `json:"sourceInfo,omitempty"`
	LicenseConcluded string            `json:"licenseConcluded"`
	LicenseDeclared  string            `json:"licenseDeclared"`
	CopyrightText    string            `json:"copyrightText"`
	ExternalRefs     []SPDXExternalRef `json:"externalRefs,omitempty"`
	Supplier         string            `json:"supplier,omitempty"`
	Version          string            `json:"versionInfo,omitempty"`
}

// SPDXVerification represents verification code in SPDX
type SPDXVerification struct {
	Value         string   `json:"packageVerificationCodeValue"`
	ExcludedFiles []string `json:"packageVerificationCodeExcludedFiles,omitempty"`
}

// SPDXChecksum represents a checksum in SPDX format
type SPDXChecksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"checksumValue"`
}

// SPDXExternalRef represents an external reference in SPDX
type SPDXExternalRef struct {
	Category string `json:"referenceCategory"`
	Type     string `json:"referenceType"`
	Locator  string `json:"referenceLocator"`
}

// SPDXRelationship represents a relationship between SPDX elements
type SPDXRelationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

// SPDXExternalDocRef represents an external document reference
type SPDXExternalDocRef struct {
	ExternalDocumentID string       `json:"externalDocumentId"`
	Checksum           SPDXChecksum `json:"checksum"`
	SPDXDocument       string       `json:"spdxDocument"`
}

// SBOMGenerator generates Software Bill of Materials
type SBOMGenerator struct {
	logger        zerolog.Logger
	scannerBinary string // syft, trivy, etc.
	tempDir       string
}

// NewSBOMGenerator creates a new SBOM generator
func NewSBOMGenerator(logger zerolog.Logger) *SBOMGenerator {
	return &SBOMGenerator{
		logger:        logger.With().Str("component", "sbom_generator").Logger(),
		scannerBinary: "syft", // Default to syft for comprehensive SBOM generation
		tempDir:       os.TempDir(),
	}
}

// SetScannerBinary sets the scanner binary to use
func (s *SBOMGenerator) SetScannerBinary(binary string) {
	s.scannerBinary = binary
}

// GenerateSBOM generates an SBOM for the given source
func (s *SBOMGenerator) GenerateSBOM(ctx context.Context, source string, format SBOMFormat) (interface{}, error) {
	s.logger.Info().
		Str("source", source).
		Str("format", string(format)).
		Msg("Generating SBOM")

	startTime := time.Now()

	// For now, we'll implement a basic SBOM generation
	// In a real implementation, this would call syft or another tool
	packages, err := s.discoverPackages(ctx, source)
	if err != nil {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "core", "failed to discover packages", err)
	}

	var result interface{}

	switch format {
	case SBOMFormatSPDX:
		result = s.createSPDXDocument(source, packages)
	case SBOMFormatCycloneDX:
		result = s.GenerateCycloneDXBOM(source, packages)
	default:
		return nil, mcperrors.New(mcperrors.CodeInternalError, "core", fmt.Sprintf("unsupported SBOM format: %s", format), nil)
	}

	duration := time.Since(startTime)
	s.logger.Info().
		Dur("duration", duration).
		Int("packages", len(packages)).
		Msg("SBOM generation completed")

	return result, nil
}

// discoverPackages discovers packages in the source
func (s *SBOMGenerator) discoverPackages(ctx context.Context, source string) ([]Package, error) {
	var packages []Package

	// Check if source is a directory or container image
	info, err := os.Stat(source)
	if err == nil && info.IsDir() {
		// Scan directory for package files
		packages, err = s.scanDirectory(ctx, source)
		if err != nil {
			return nil, err
		}
	} else {
		// Assume it's a container image
		packages, err = s.scanContainerImage(ctx, source)
		if err != nil {
			return nil, err
		}
	}

	return packages, nil
}

// scanDirectory scans a directory for packages
func (s *SBOMGenerator) scanDirectory(_ context.Context, dir string) ([]Package, error) {
	var packages []Package

	// Look for common package files
	packageFiles := map[string]string{
		"package.json":      "npm",
		"package-lock.json": "npm",
		"yarn.lock":         "yarn",
		"requirements.txt":  "pip",
		"Pipfile":           "pipenv",
		"go.mod":            "go",
		"go.sum":            "go",
		"Gemfile":           "gem",
		"Gemfile.lock":      "gem",
		"pom.xml":           "maven",
		"build.gradle":      "gradle",
		"Cargo.toml":        "cargo",
		"Cargo.lock":        "cargo",
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		filename := filepath.Base(path)
		if packageType, exists := packageFiles[filename]; exists {
			s.logger.Debug().
				Str("file", path).
				Str("type", packageType).
				Msg("Found package file")

			// Parse packages from file
			filePkgs, err := s.parsePackageFile(path, packageType)
			if err != nil {
				s.logger.Warn().
					Err(err).
					Str("file", path).
					Msg("Failed to parse package file")
				return nil // Continue scanning
			}

			packages = append(packages, filePkgs...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return packages, nil
}

// scanContainerImage scans a container image for packages
func (s *SBOMGenerator) scanContainerImage(_ context.Context, image string) ([]Package, error) {
	// This is a placeholder - in real implementation, would use syft or similar
	s.logger.Info().Str("image", image).Msg("Scanning container image for packages")

	// For now, return empty list
	return []Package{}, nil
}

// parsePackageFile parses packages from a package file
func (s *SBOMGenerator) parsePackageFile(path string, packageType string) ([]Package, error) {
	var packages []Package

	// Read file content
	// nolint:gosec // Path is controlled by the function caller
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	switch packageType {
	case "npm":
		if strings.HasSuffix(path, "package.json") {
			pkg, err := s.parsePackageJSON(content)
			if err != nil {
				return nil, err
			}
			packages = append(packages, pkg)
		}
	case "go":
		if strings.HasSuffix(path, "go.mod") {
			pkgs, err := s.parseGoMod(content)
			if err != nil {
				return nil, err
			}
			packages = append(packages, pkgs...)
		}
	case "pip":
		if strings.HasSuffix(path, "requirements.txt") {
			pkgs, err := s.parseRequirementsTxt(content)
			if err != nil {
				return nil, err
			}
			packages = append(packages, pkgs...)
		}
	}

	// Calculate checksums for the package file
	checksum := s.calculateChecksum(content)
	for i := range packages {
		if packages[i].Checksums == nil {
			packages[i].Checksums = make(map[string]string)
		}
		packages[i].Checksums["SHA256"] = checksum
		packages[i].FilesScanned = append(packages[i].FilesScanned, path)
	}

	return packages, nil
}

// parsePackageJSON parses a package.json file
func (s *SBOMGenerator) parsePackageJSON(content []byte) (Package, error) {
	var pkgJSON struct {
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		License      string            `json:"license"`
		Author       string            `json:"author"`
		Dependencies map[string]string `json:"dependencies"`
	}

	if err := json.Unmarshal(content, &pkgJSON); err != nil {
		return Package{}, fmt.Errorf("failed to parse package.json: %w", err)
	}

	pkg := Package{
		Name:     pkgJSON.Name,
		Version:  pkgJSON.Version,
		Type:     "npm",
		License:  pkgJSON.License,
		Supplier: pkgJSON.Author,
		PURL:     fmt.Sprintf("pkg:npm/%s@%s", pkgJSON.Name, pkgJSON.Version),
	}

	// Add dependencies
	for dep, version := range pkgJSON.Dependencies {
		pkg.Dependencies = append(pkg.Dependencies, fmt.Sprintf("%s@%s", dep, version))
	}

	return pkg, nil
}

// parseGoMod parses a go.mod file
func (s *SBOMGenerator) parseGoMod(content []byte) ([]Package, error) {
	var packages []Package
	lines := strings.Split(string(content), "\n")

	var moduleName string
	inRequire := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "module ") {
			moduleName = strings.TrimPrefix(line, "module ")
			continue
		}

		if line == "require (" {
			inRequire = true
			continue
		}

		if line == ")" {
			inRequire = false
			continue
		}

		if inRequire && line != "" && !strings.HasPrefix(line, "//") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]

				pkg := Package{
					Name:    name,
					Version: version,
					Type:    "go",
					PURL:    fmt.Sprintf("pkg:golang/%s@%s", name, version),
				}
				packages = append(packages, pkg)
			}
		}
	}

	// Add the main module
	if moduleName != "" {
		mainPkg := Package{
			Name: moduleName,
			Type: "go",
			PURL: fmt.Sprintf("pkg:golang/%s", moduleName),
		}
		packages = append(packages, mainPkg)
	}

	return packages, nil
}

// parseRequirementsTxt parses a requirements.txt file
func (s *SBOMGenerator) parseRequirementsTxt(content []byte) ([]Package, error) {
	var packages []Package
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse package==version format
		parts := strings.Split(line, "==")
		if len(parts) == 2 {
			pkg := Package{
				Name:    parts[0],
				Version: parts[1],
				Type:    "pip",
				PURL:    fmt.Sprintf("pkg:pypi/%s@%s", parts[0], parts[1]),
			}
			packages = append(packages, pkg)
		}
	}

	return packages, nil
}

// calculateChecksum calculates SHA256 checksum
func (s *SBOMGenerator) calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// createSPDXDocument creates an SPDX document from packages
func (s *SBOMGenerator) createSPDXDocument(source string, packages []Package) *SPDXDocument {
	now := time.Now().UTC().Format(time.RFC3339)

	doc := &SPDXDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              fmt.Sprintf("SBOM for %s", source),
		DocumentNamespace: fmt.Sprintf("https://sbom.example.com/%s-%d", source, time.Now().Unix()),
		CreationInfo: SPDXCreationInfo{
			Created:            now,
			Creators:           []string{"Tool: container-kit-security-scanner"},
			LicenseListVersion: "3.19",
		},
		Packages:      []SPDXPackage{},
		Relationships: []SPDXRelationship{},
	}

	// Convert packages to SPDX format
	for i, pkg := range packages {
		spdxPkg := SPDXPackage{
			SPDXID:           fmt.Sprintf("SPDXRef-Package-%d", i),
			Name:             pkg.Name,
			Version:          pkg.Version,
			DownloadLocation: "NOASSERTION",
			FilesAnalyzed:    false,
			LicenseConcluded: s.normalizeLicense(pkg.License),
			LicenseDeclared:  s.normalizeLicense(pkg.License),
			CopyrightText:    "NOASSERTION",
		}

		// Add checksums
		if pkg.Checksums != nil {
			for algo, value := range pkg.Checksums {
				spdxPkg.Checksums = append(spdxPkg.Checksums, SPDXChecksum{
					Algorithm: algo,
					Value:     value,
				})
			}
		}

		// Add external references
		if pkg.PURL != "" {
			spdxPkg.ExternalRefs = append(spdxPkg.ExternalRefs, SPDXExternalRef{
				Category: "PACKAGE-MANAGER",
				Type:     "purl",
				Locator:  pkg.PURL,
			})
		}

		if pkg.CPE != "" {
			spdxPkg.ExternalRefs = append(spdxPkg.ExternalRefs, SPDXExternalRef{
				Category: "SECURITY",
				Type:     "cpe23Type",
				Locator:  pkg.CPE,
			})
		}

		if pkg.Supplier != "" {
			spdxPkg.Supplier = fmt.Sprintf("Organization: %s", pkg.Supplier)
		}

		doc.Packages = append(doc.Packages, spdxPkg)

		// Add relationship
		doc.Relationships = append(doc.Relationships, SPDXRelationship{
			SPDXElementID:      "SPDXRef-DOCUMENT",
			RelationshipType:   "DESCRIBES",
			RelatedSPDXElement: spdxPkg.SPDXID,
		})
	}

	return doc
}

// normalizeLicense normalizes license string to SPDX format
func (s *SBOMGenerator) normalizeLicense(license string) string {
	if license == "" {
		return "NOASSERTION"
	}

	// Common license mappings
	licenseMap := map[string]string{
		"MIT":          "MIT",
		"Apache-2.0":   "Apache-2.0",
		"Apache 2.0":   "Apache-2.0",
		"GPL-3.0":      "GPL-3.0-only",
		"GPL-2.0":      "GPL-2.0-only",
		"BSD-3-Clause": "BSD-3-Clause",
		"BSD-2-Clause": "BSD-2-Clause",
		"ISC":          "ISC",
		"MPL-2.0":      "MPL-2.0",
		"LGPL-3.0":     "LGPL-3.0-only",
		"LGPL-2.1":     "LGPL-2.1-only",
	}

	if normalized, exists := licenseMap[license]; exists {
		return normalized
	}

	// If not found, return as-is
	return license
}

// WriteSPDX writes SPDX document to a writer
func (s *SBOMGenerator) WriteSPDX(doc *SPDXDocument, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

// ValidateSBOM validates an SBOM document
func (s *SBOMGenerator) ValidateSBOM(doc *SPDXDocument) error {
	// Basic validation
	if doc.SPDXVersion == "" {
		return mcperrors.New(mcperrors.CodeValidationFailed, "core", "SPDX version is required", nil)
	}

	if doc.DataLicense == "" {
		return mcperrors.New(mcperrors.CodeValidationFailed, "core", "data license is required", nil)
	}

	if doc.Name == "" {
		return mcperrors.New(mcperrors.CodeValidationFailed, "core", "document name is required", nil)
	}

	if doc.DocumentNamespace == "" {
		return mcperrors.New(mcperrors.CodeValidationFailed, "core", "document namespace is required", nil)
	}

	if len(doc.Packages) == 0 {
		return mcperrors.New(mcperrors.CodeValidationFailed, "core", "at least one package is required", nil)
	}

	for i, pkg := range doc.Packages {
		if pkg.SPDXID == "" {
			return mcperrors.New(mcperrors.CodeValidationFailed, "core", "package %d: SPDX ID is required", nil)
		}
		if pkg.Name == "" {
			return mcperrors.New(mcperrors.CodeValidationFailed, "core", "package %d: name is required", nil)
		}
		if pkg.DownloadLocation == "" {
			return mcperrors.New(mcperrors.CodeInternalError, "security", fmt.Sprintf("package %d: download location is required", i), nil)
		}
	}

	packageIDs := make(map[string]bool)
	packageIDs["SPDXRef-DOCUMENT"] = true
	for _, pkg := range doc.Packages {
		packageIDs[pkg.SPDXID] = true
	}

	for i, rel := range doc.Relationships {
		if !packageIDs[rel.SPDXElementID] {
			return mcperrors.New(mcperrors.CodeValidationFailed, "core", fmt.Sprintf("relationship %d: invalid SPDX element ID: %s", i, rel.SPDXElementID), nil)
		}
		if !packageIDs[rel.RelatedSPDXElement] {
			return mcperrors.New(mcperrors.CodeValidationFailed, "core", fmt.Sprintf("relationship %d: invalid related SPDX element: %s", i, rel.RelatedSPDXElement), nil)
		}
	}

	return nil
}
