package core

import (
	"sync"
)

// RegistryService provides container registry management without global state
type RegistryService struct {
	mu              sync.RWMutex
	knownRegistries []string
}

// NewRegistryService creates a new registry service
func NewRegistryService() *RegistryService {
	return &RegistryService{
		knownRegistries: getDefaultKnownRegistries(),
	}
}

// getDefaultKnownRegistries returns the default list of known registries
func getDefaultKnownRegistries() []string {
	return []string{
		"docker.io",
		"gcr.io",
		"quay.io",
		"ghcr.io",
	}
}

// GetKnownRegistries returns a copy of all known registries
func (rs *RegistryService) GetKnownRegistries() []string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]string, len(rs.knownRegistries))
	copy(result, rs.knownRegistries)
	return result
}

// AddRegistry adds a new registry to the known registries list
func (rs *RegistryService) AddRegistry(registry string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Check if registry already exists
	for _, existing := range rs.knownRegistries {
		if existing == registry {
			return // Already exists
		}
	}

	rs.knownRegistries = append(rs.knownRegistries, registry)
}

// RemoveRegistry removes a registry from the known registries list
func (rs *RegistryService) RemoveRegistry(registry string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for i, existing := range rs.knownRegistries {
		if existing == registry {
			rs.knownRegistries = append(rs.knownRegistries[:i], rs.knownRegistries[i+1:]...)
			return true
		}
	}

	return false // Not found
}

// IsKnownRegistry checks if a registry is in the known registries list
func (rs *RegistryService) IsKnownRegistry(registry string) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	for _, known := range rs.knownRegistries {
		if known == registry {
			return true
		}
	}

	return false
}

// SetKnownRegistries sets the entire list of known registries
func (rs *RegistryService) SetKnownRegistries(registries []string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Create a copy to prevent external modification
	rs.knownRegistries = make([]string, len(registries))
	copy(rs.knownRegistries, registries)
}

// Reset resets the service to default known registries
func (rs *RegistryService) Reset() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.knownRegistries = getDefaultKnownRegistries()
}
