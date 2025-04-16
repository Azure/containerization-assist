package docker

import (
	"fmt"
	"net/http"
)

// validateRegistryReachable checks if the local Docker registry is reachable.
func ValidateRegistryReachable(registryURL string) error {
	resp, err := http.Get(fmt.Sprintf("http://%s/v2/", registryURL))
	if err != nil {
		return fmt.Errorf("failed to reach local registry at %s: %w", registryURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("unexpected response from registry: %d", resp.StatusCode)
	}
	return nil
}
