// Package sampling provides factory methods for creating sampling clients with middleware
package sampling

import (
	"log/slog"

	"github.com/Azure/containerization-assist/pkg/domain/sampling"
)

// CreateDomainClient creates a domain-compatible client
func CreateDomainClient(logger *slog.Logger) sampling.UnifiedSampler {
	// Return the client directly since it now implements UnifiedSampler interface
	return NewClient(logger)
}

// domainAdapter removed - Client now directly implements UnifiedSampler interface
