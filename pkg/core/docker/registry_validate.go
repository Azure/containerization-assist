package docker

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// validateRegistryReachable checks if the local Docker registry is reachable.
func ValidateRegistryReachable(ctx context.Context, registryURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/v2/", registryURL), nil)
	if err != nil {
		return errors.New(errors.CodeNetworkError, "docker", fmt.Sprintf("failed to create request: %v", err), err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.New(errors.CodeNetworkError, "docker", fmt.Sprintf("failed to reach local registry at %s: %v", registryURL, err), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return errors.New(errors.CodeNetworkError, "docker", fmt.Sprintf("unexpected response from registry: %d", resp.StatusCode), nil)
	}
	return nil
}
