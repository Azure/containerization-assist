package observability

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/rs/zerolog"
)

// SecurityValidator handles security-related checks
type SecurityValidator struct {
	logger zerolog.Logger
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(logger zerolog.Logger) *SecurityValidator {
	return &SecurityValidator{
		logger: logger,
	}
}

// GetSecurityCheck returns a pre-flight check for security vulnerabilities
func (sv *SecurityValidator) GetSecurityCheck(state *session.SessionState) PreFlightCheck {
	return PreFlightCheck{
		Name:        "Security vulnerabilities",
		Description: "Ensure image has no critical vulnerabilities",
		Category:    "security",
		CheckFunc: func(ctx context.Context) error {
			if state.SecurityScan.Summary.Critical > 0 {
				return fmt.Errorf("image has %d CRITICAL vulnerabilities", state.SecurityScan.Summary.Critical)
			}
			if state.SecurityScan.Summary.High > 3 {
				return fmt.Errorf("image has %d HIGH vulnerabilities (threshold: 3)", state.SecurityScan.Summary.High)
			}
			return nil
		},
		ErrorRecovery: "Fix critical vulnerabilities before pushing to registry",
		Optional:      false,
	}
}
