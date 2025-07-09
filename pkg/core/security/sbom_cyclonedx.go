// Package security provides CycloneDX format support for SBOM generation
package security

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/google/uuid"
)

// CycloneDXBOM represents a CycloneDX BOM document
type CycloneDXBOM struct {
	BOMFormat    string                `json:"bomFormat"`
	SpecVersion  string                `json:"specVersion"`
	SerialNumber string                `json:"serialNumber"`
	Version      int                   `json:"version"`
	Metadata     CycloneDXMetadata     `json:"metadata"`
	Components   []CycloneDXComponent  `json:"components"`
	Dependencies []CycloneDXDependency `json:"dependencies,omitempty"`
}

// CycloneDXMetadata represents metadata in CycloneDX format
type CycloneDXMetadata struct {
	Timestamp string              `json:"timestamp"`
	Tools     []CycloneDXTool     `json:"tools"`
	Component *CycloneDXComponent `json:"component,omitempty"`
}

// CycloneDXTool represents a tool in CycloneDX format
type CycloneDXTool struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// CycloneDXComponent represents a component in CycloneDX format
type CycloneDXComponent struct {
	Type         string                 `json:"type"`
	BOMRef       string                 `json:"bom-ref"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Scope        string                 `json:"scope,omitempty"`
	Hashes       []CycloneDXHash        `json:"hashes,omitempty"`
	Licenses     []CycloneDXLicense     `json:"licenses,omitempty"`
	PURL         string                 `json:"purl,omitempty"`
	CPE          string                 `json:"cpe,omitempty"`
	ExternalRefs []CycloneDXExternalRef `json:"externalReferences,omitempty"`
	Properties   []CycloneDXProperty    `json:"properties,omitempty"`
}

// CycloneDXHash represents a hash in CycloneDX format
type CycloneDXHash struct {
	Algorithm string `json:"alg"`
	Content   string `json:"content"`
}

// CycloneDXLicense represents a license in CycloneDX format
type CycloneDXLicense struct {
	License *CycloneDXLicenseChoice `json:"license,omitempty"`
}

// CycloneDXLicenseChoice represents license choices in CycloneDX
type CycloneDXLicenseChoice struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// CycloneDXExternalRef represents an external reference in CycloneDX
type CycloneDXExternalRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// CycloneDXDependency represents a dependency in CycloneDX format
type CycloneDXDependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

// CycloneDXProperty represents a property in CycloneDX format
type CycloneDXProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// GenerateCycloneDXBOM generates a CycloneDX BOM from packages
func (s *SBOMGenerator) GenerateCycloneDXBOM(source string, packages []Package) *CycloneDXBOM {
	serialNumber := fmt.Sprintf("urn:uuid:%s", uuid.New().String())

	bom := &CycloneDXBOM{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.4",
		SerialNumber: serialNumber,
		Version:      1,
		Metadata: CycloneDXMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tools: []CycloneDXTool{
				{
					Vendor:  "container-kit",
					Name:    "security-scanner",
					Version: "1.0.0",
				},
			},
		},
		Components:   []CycloneDXComponent{},
		Dependencies: []CycloneDXDependency{},
	}

	// Add main component if source is specified
	if source != "" {
		mainComponent := CycloneDXComponent{
			Type:   "application",
			BOMRef: serialNumber,
			Name:   source,
		}
		bom.Metadata.Component = &mainComponent
	}

	// Convert packages to CycloneDX components
	dependencyMap := make(map[string][]string)

	for _, pkg := range packages {
		bomRef := fmt.Sprintf("pkg:%s/%s@%s", pkg.Type, pkg.Name, pkg.Version)

		component := CycloneDXComponent{
			Type:    s.getComponentType(pkg.Type),
			BOMRef:  bomRef,
			Name:    pkg.Name,
			Version: pkg.Version,
			Scope:   "required",
			PURL:    pkg.PURL,
			CPE:     pkg.CPE,
		}

		// Add hashes
		if pkg.Checksums != nil {
			for algo, value := range pkg.Checksums {
				component.Hashes = append(component.Hashes, CycloneDXHash{
					Algorithm: s.normalizeCycloneDXHashAlgo(algo),
					Content:   value,
				})
			}
		}

		// Add licenses
		if pkg.License != "" {
			license := s.createCycloneDXLicense(pkg.License)
			component.Licenses = append(component.Licenses, license)
		}

		// Add external references
		if pkg.Metadata != nil {
			if homepage, exists := pkg.Metadata["homepage"]; exists {
				component.ExternalRefs = append(component.ExternalRefs, CycloneDXExternalRef{
					Type: "website",
					URL:  homepage,
				})
			}
			if vcs, exists := pkg.Metadata["vcs"]; exists {
				component.ExternalRefs = append(component.ExternalRefs, CycloneDXExternalRef{
					Type: "vcs",
					URL:  vcs,
				})
			}
		}

		// Add properties for additional metadata
		if pkg.Supplier != "" {
			component.Properties = append(component.Properties, CycloneDXProperty{
				Name:  "supplier",
				Value: pkg.Supplier,
			})
		}

		bom.Components = append(bom.Components, component)

		// Build dependency map
		if len(pkg.Dependencies) > 0 {
			deps := make([]string, 0, len(pkg.Dependencies))
			for _, dep := range pkg.Dependencies {
				// Convert dependency to BOM ref format
				deps = append(deps, s.dependencyToBomRef(dep, pkg.Type))
			}
			dependencyMap[bomRef] = deps
		}
	}

	// Add dependencies
	for ref, deps := range dependencyMap {
		bom.Dependencies = append(bom.Dependencies, CycloneDXDependency{
			Ref:       ref,
			DependsOn: deps,
		})
	}

	// Add root dependency if we have a main component
	if bom.Metadata.Component != nil && len(bom.Components) > 0 {
		rootDeps := make([]string, 0, len(bom.Components))
		for _, comp := range bom.Components {
			rootDeps = append(rootDeps, comp.BOMRef)
		}
		bom.Dependencies = append(bom.Dependencies, CycloneDXDependency{
			Ref:       serialNumber,
			DependsOn: rootDeps,
		})
	}

	return bom
}

// getComponentType maps package type to CycloneDX component type
func (s *SBOMGenerator) getComponentType(packageType string) string {
	switch packageType {
	case "npm", "pip", "gem", "go", "maven", "gradle", "cargo":
		return "library"
	case "container":
		return "container"
	case "os":
		return "operating-system"
	default:
		return "library"
	}
}

// normalizeCycloneDXHashAlgo normalizes hash algorithm names for CycloneDX
func (s *SBOMGenerator) normalizeCycloneDXHashAlgo(algo string) string {
	switch algo {
	case "SHA256", "sha256":
		return "SHA-256"
	case "SHA512", "sha512":
		return "SHA-512"
	case "SHA1", "sha1":
		return "SHA-1"
	case "MD5", "md5":
		return "MD5"
	default:
		return algo
	}
}

// createCycloneDXLicense creates a CycloneDX license object
func (s *SBOMGenerator) createCycloneDXLicense(license string) CycloneDXLicense {
	// Normalize to SPDX ID
	spdxID := s.normalizeLicense(license)

	licenseChoice := &CycloneDXLicenseChoice{}

	// If it's a known SPDX license (normalized ID is different and not NOASSERTION), use ID
	// OR if the original license is already a known SPDX ID, use it as ID
	if (spdxID != license && spdxID != "NOASSERTION") || s.isKnownSPDXLicense(license) {
		licenseChoice.ID = spdxID
	} else {
		// Otherwise use name
		licenseChoice.Name = license
	}

	return CycloneDXLicense{
		License: licenseChoice,
	}
}

// isKnownSPDXLicense checks if a license string is a known SPDX license
func (s *SBOMGenerator) isKnownSPDXLicense(license string) bool {
	knownSPDXLicenses := map[string]bool{
		"MIT":           true,
		"Apache-2.0":    true,
		"GPL-3.0-only":  true,
		"GPL-2.0-only":  true,
		"BSD-3-Clause":  true,
		"BSD-2-Clause":  true,
		"ISC":           true,
		"MPL-2.0":       true,
		"LGPL-3.0-only": true,
		"LGPL-2.1-only": true,
	}
	return knownSPDXLicenses[license]
}

// dependencyToBomRef converts a dependency string to BOM ref format
func (s *SBOMGenerator) dependencyToBomRef(dep string, packageType string) string {
	// Handle different dependency formats
	// npm: "package@version"
	// go: "github.com/user/repo@version"
	// pip: "package==version"

	var name, version string

	switch packageType {
	case "npm", "go":
		if idx := strings.LastIndex(dep, "@"); idx > 0 {
			name = dep[:idx]
			version = dep[idx+1:]
		} else {
			name = dep
		}
	case "pip":
		if idx := strings.Index(dep, "=="); idx > 0 {
			name = dep[:idx]
			version = dep[idx+2:]
		} else {
			name = dep
		}
	default:
		name = dep
	}

	if version != "" {
		return fmt.Sprintf("pkg:%s/%s@%s", packageType, name, version)
	}
	return fmt.Sprintf("pkg:%s/%s", packageType, name)
}

// WriteCycloneDX writes CycloneDX BOM to a writer
func (s *SBOMGenerator) WriteCycloneDX(bom *CycloneDXBOM, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(bom)
}

// ValidateCycloneDXBOM validates a CycloneDX BOM document
func (s *SBOMGenerator) ValidateCycloneDXBOM(bom *CycloneDXBOM) error {
	// Basic validation
	if bom.BOMFormat != "CycloneDX" {
		return mcperrors.NewError().Messagef("invalid BOM format: %s", bom.BOMFormat).WithLocation().Build()
	}

	if bom.SpecVersion == "" {
		return mcperrors.NewError().Messagef("spec version is required").WithLocation().Build()
	}

	if bom.SerialNumber == "" {
		return mcperrors.NewError().Messagef("serial number is required").WithLocation().Build()
	}

	if bom.Version < 1 {
		return mcperrors.NewError().Messagef("version must be >= 1").WithLocation(

		// Validate components
		).Build()
	}

	bomRefs := make(map[string]bool)
	for i, component := range bom.Components {
		if component.Type == "" {
			return mcperrors.NewError().Messagef("component %d: type is required", i).WithLocation().Build()
		}
		if component.BOMRef == "" {
			return mcperrors.NewError().Messagef("component %d: bom-ref is required", i).WithLocation().Build()
		}
		if component.Name == "" {
			return mcperrors.NewError().Messagef("component %d: name is required", i).WithLocation(

			// Check for duplicate BOM refs
			).Build()
		}

		if bomRefs[component.BOMRef] {
			return mcperrors.NewError().Messagef("duplicate bom-ref: %s", component.BOMRef).WithLocation().Build()
		}
		bomRefs[component.BOMRef] = true
	}

	// Add main component BOM ref if present
	if bom.Metadata.Component != nil {
		bomRefs[bom.SerialNumber] = true
	}

	// Validate dependencies
	for i, dep := range bom.Dependencies {
		if !bomRefs[dep.Ref] {
			return mcperrors.NewError().Messagef("dependency %d: unknown ref: %s", i, dep.Ref).WithLocation().Build()
		}

		for j, depRef := range dep.DependsOn {
			if !bomRefs[depRef] {
				return mcperrors.NewError().Messagef("dependency %d, dependsOn %d: unknown ref: %s", i, j, depRef).WithLocation().Build()
			}
		}
	}

	return nil
}

// ConvertSPDXToCycloneDX converts an SPDX document to CycloneDX format
func (s *SBOMGenerator) ConvertSPDXToCycloneDX(spdx *SPDXDocument) (*CycloneDXBOM, error) {
	serialNumber := fmt.Sprintf("urn:uuid:%s", uuid.New().String())

	bom := &CycloneDXBOM{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.4",
		SerialNumber: serialNumber,
		Version:      1,
		Metadata: CycloneDXMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tools: []CycloneDXTool{
				{
					Vendor:  "container-kit",
					Name:    "security-scanner",
					Version: "1.0.0",
				},
			},
		},
		Components:   []CycloneDXComponent{},
		Dependencies: []CycloneDXDependency{},
	}

	// Convert SPDX packages to CycloneDX components
	spdxRefToBomRef := make(map[string]string)

	for _, spdxPkg := range spdx.Packages {
		// Extract package type from external refs
		var purl, cpe string
		for _, ref := range spdxPkg.ExternalRefs {
			switch ref.Type {
			case "purl":
				purl = ref.Locator
			case "cpe23Type":
				cpe = ref.Locator
			}
		}

		bomRef := purl
		if bomRef == "" {
			bomRef = fmt.Sprintf("pkg:generic/%s@%s", spdxPkg.Name, spdxPkg.Version)
		}

		spdxRefToBomRef[spdxPkg.SPDXID] = bomRef

		component := CycloneDXComponent{
			Type:    "library", // Default to library
			BOMRef:  bomRef,
			Name:    spdxPkg.Name,
			Version: spdxPkg.Version,
			Scope:   "required",
			PURL:    purl,
			CPE:     cpe,
		}

		// Convert checksums
		for _, checksum := range spdxPkg.Checksums {
			component.Hashes = append(component.Hashes, CycloneDXHash{
				Algorithm: s.normalizeCycloneDXHashAlgo(checksum.Algorithm),
				Content:   checksum.Value,
			})
		}

		// Convert license
		if spdxPkg.LicenseDeclared != "" && spdxPkg.LicenseDeclared != "NOASSERTION" {
			component.Licenses = append(component.Licenses, CycloneDXLicense{
				License: &CycloneDXLicenseChoice{
					ID: spdxPkg.LicenseDeclared,
				},
			})
		}

		bom.Components = append(bom.Components, component)
	}

	// Convert relationships to dependencies
	dependencyMap := make(map[string][]string)

	for _, rel := range spdx.Relationships {
		if rel.RelationshipType == "DEPENDS_ON" || rel.RelationshipType == "CONTAINS" {
			fromRef := spdxRefToBomRef[rel.SPDXElementID]
			toRef := spdxRefToBomRef[rel.RelatedSPDXElement]

			if fromRef != "" && toRef != "" {
				dependencyMap[fromRef] = append(dependencyMap[fromRef], toRef)
			}
		}
	}

	// Add dependencies
	for ref, deps := range dependencyMap {
		bom.Dependencies = append(bom.Dependencies, CycloneDXDependency{
			Ref:       ref,
			DependsOn: deps,
		})
	}

	return bom, nil
}
