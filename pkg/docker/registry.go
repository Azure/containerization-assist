package docker

import (
	"context"
	"fmt"
	"net/http"
)

// validateRegistryReachable checks if the local Docker registry is reachable.
func ValidateRegistryReachable(ctx context.Context, registryURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/v2/", registryURL), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach local registry at %s: %w", registryURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("unexpected response from registry: %d", resp.StatusCode)
	}
	return nil
}
