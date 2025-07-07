package scanner

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// SecurityScannerImpl implements Scanner interface
type SecurityScannerImpl struct {
	scanners map[string]ScannerBackend
}

// ScannerBackend represents a security scanning backend
type ScannerBackend interface {
	ScanImage(ctx context.Context, imageTag string, severity string) (*services.ScanResult, error)
	ScanRepository(ctx context.Context, path string) (*services.ScanResult, error)
}

// NewSecurityScanner creates a new security scanner
func NewSecurityScanner() *SecurityScannerImpl {
	return &SecurityScannerImpl{
		scanners: map[string]ScannerBackend{
			"trivy": &TrivyScanner{},
			"grype": &GrypeScanner{},
		},
	}
}

// ScanImage implements Scanner.ScanImage
func (s *SecurityScannerImpl) ScanImage(ctx context.Context, config *services.ScanConfig) (*services.ScanResult, error) {
	if config == nil {
		return nil, errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("scan configuration cannot be nil").Build()
	}

	if config.ImageTag == "" {
		return nil, errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("image tag is required for scanning").Build()
	}

	scanner, exists := s.scanners[config.Scanner]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeValidation).
			Message("scanner not supported").
			Context("scanner", config.Scanner).
			Context("available_scanners", []string{"trivy", "grype"}).Build()
	}

	result, err := scanner.ScanImage(ctx, config.ImageTag, config.Severity)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeSecurity).
			Message("image security scan failed").
			Context("image_tag", config.ImageTag).
			Context("scanner", config.Scanner).
			Cause(err).Build()
	}

	return result, nil
}

// ScanRepository implements Scanner.ScanRepository
func (s *SecurityScannerImpl) ScanRepository(ctx context.Context, config *services.RepoScanConfig) (*services.ScanResult, error) {
	if config == nil {
		return nil, errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("repository scan configuration cannot be nil").Build()
	}

	if config.Path == "" {
		return nil, errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("repository path is required for scanning").Build()
	}

	scanner, exists := s.scanners[config.Scanner]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeValidation).
			Message("scanner not supported").
			Context("scanner", config.Scanner).
			Context("available_scanners", []string{"trivy", "grype"}).Build()
	}

	result, err := scanner.ScanRepository(ctx, config.Path)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeSecurity).
			Message("repository security scan failed").
			Context("path", config.Path).
			Context("scanner", config.Scanner).
			Cause(err).Build()
	}

	return result, nil
}

// GetSecurityReport implements Scanner.GetSecurityReport
func (s *SecurityScannerImpl) GetSecurityReport(ctx context.Context, target string) (*services.ScanResult, error) {
	if target == "" {
		return nil, errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("scan target is required").Build()
	}

	scanner := s.scanners["trivy"]

	result, err := scanner.ScanImage(ctx, target, "HIGH")
	if err != nil {
		result, err = scanner.ScanRepository(ctx, target)
		if err != nil {
			return nil, errors.NewError().
				Code(errors.CodeInternalError).
				Type(errors.ErrTypeSecurity).
				Message("security report generation failed").
				Context("target", target).
				Cause(err).Build()
		}
	}

	return result, nil
}

// TrivyScanner implements ScannerBackend for Trivy
type TrivyScanner struct{}

func (t *TrivyScanner) ScanImage(ctx context.Context, imageTag string, severity string) (*services.ScanResult, error) {
	return &services.ScanResult{
		Vulnerabilities: []services.Vulnerability{
			{
				ID:          "CVE-2023-1234",
				Severity:    "HIGH",
				Package:     "example-package",
				Version:     "1.0.0",
				FixedIn:     "1.0.1",
				Description: "Example vulnerability from Trivy scan",
			},
		},
		Summary: services.ScanSummary{
			Total:    1,
			Critical: 0,
			High:     1,
			Medium:   0,
			Low:      0,
		},
	}, nil
}

func (t *TrivyScanner) ScanRepository(ctx context.Context, path string) (*services.ScanResult, error) {
	return &services.ScanResult{
		Vulnerabilities: []services.Vulnerability{
			{
				ID:          "CVE-2023-5678",
				Severity:    "MEDIUM",
				Package:     "dependency-package",
				Version:     "2.0.0",
				FixedIn:     "2.0.1",
				Description: "Example repository vulnerability from Trivy scan",
			},
		},
		Summary: services.ScanSummary{
			Total:    1,
			Critical: 0,
			High:     0,
			Medium:   1,
			Low:      0,
		},
	}, nil
}

// GrypeScanner implements ScannerBackend for Grype
type GrypeScanner struct{}

func (g *GrypeScanner) ScanImage(ctx context.Context, imageTag string, severity string) (*services.ScanResult, error) {
	return &services.ScanResult{
		Vulnerabilities: []services.Vulnerability{
			{
				ID:          "GHSA-abcd-1234",
				Severity:    "HIGH",
				Package:     "grype-example",
				Version:     "0.9.0",
				FixedIn:     "1.0.0",
				Description: "Example vulnerability from Grype scan",
			},
		},
		Summary: services.ScanSummary{
			Total:    1,
			Critical: 0,
			High:     1,
			Medium:   0,
			Low:      0,
		},
	}, nil
}

func (g *GrypeScanner) ScanRepository(ctx context.Context, path string) (*services.ScanResult, error) {
	return &services.ScanResult{
		Vulnerabilities: []services.Vulnerability{
			{
				ID:          "GHSA-efgh-5678",
				Severity:    "LOW",
				Package:     "grype-repo-pkg",
				Version:     "1.5.0",
				FixedIn:     "1.5.1",
				Description: "Example repository vulnerability from Grype scan",
			},
		},
		Summary: services.ScanSummary{
			Total:    1,
			Critical: 0,
			High:     0,
			Medium:   0,
			Low:      1,
		},
	}, nil
}

// Close closes the security scanner
func (s *SecurityScannerImpl) Close() error {
	return nil
}
