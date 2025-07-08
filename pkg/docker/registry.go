package docker

import (
	"context"
	"fmt"
	"net/http"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// validateRegistryReachable checks if the local Docker registry is reachable.
func ValidateRegistryReachable(ctx context.Context, registryURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/v2/", registryURL), nil)
	if err != nil {
		return mcperrors.NewError().Messagef("failed to create request: %w", err).WithLocation().Build()
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return mcperrors.NewError().Messagef("failed to reach local registry at %s: %w", registryURL, err).WithLocation().Build()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return mcperrors.NewError().Messagef("unexpected response from registry: %d", resp.StatusCode).WithLocation().Build()
	}
	return nil
}
