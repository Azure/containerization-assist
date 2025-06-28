package registry

import "time"

// RegistryCredentials represents credentials for a container registry
type RegistryCredentials struct {
	Username   string
	Password   string
	Token      string
	ExpiresAt  *time.Time
	Registry   string
	AuthMethod string
	Source     string // Which provider returned these credentials
}
