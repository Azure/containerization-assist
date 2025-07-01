package observability

import (
	"context"
	"time"
)

// DockerConfig represents the structure of Docker's config.json file
type DockerConfig struct {
	Auths             map[string]DockerAuth `json:"auths"`
	CredHelpers       map[string]string     `json:"credHelpers,omitempty"`
	CredsStore        string                `json:"credsStore,omitempty"`
	CredentialHelpers map[string]string     `json:"credentialHelpers,omitempty"`
}

// DockerAuth represents authentication information for a registry
type DockerAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

// RegistryAuthInfo contains parsed authentication information for a registry
type RegistryAuthInfo struct {
	Registry string
	Username string
	HasAuth  bool
	AuthType string
	Helper   string
}

// RegistryAuthSummary contains authentication status for all configured registries
type RegistryAuthSummary struct {
	ConfigPath    string
	Registries    []RegistryAuthInfo
	DefaultHelper string
	HasStore      bool
}

// PreFlightCheck represents a single validation check
type PreFlightCheck struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	CheckFunc     func(context.Context) error
	ErrorRecovery string `json:"error_recovery"`
	Optional      bool   `json:"optional"`
	Category      string `json:"category"`
}

// PreFlightResult contains the results of all pre-flight checks
type PreFlightResult struct {
	Passed      bool              `json:"passed"`
	Timestamp   time.Time         `json:"timestamp"`
	Duration    time.Duration     `json:"duration"`
	Checks      []CheckResult     `json:"checks"`
	Suggestions map[string]string `json:"suggestions"`
	CanProceed  bool              `json:"can_proceed"`
}

// CheckResult represents the result of a single check
type CheckResult struct {
	Name           string        `json:"name"`
	Category       string        `json:"category"`
	Status         CheckStatus   `json:"status"`
	Message        string        `json:"message"`
	Error          string        `json:"error,omitempty"`
	Duration       time.Duration `json:"duration"`
	RecoveryAction string        `json:"recovery_action,omitempty"`
}

// CheckStatus represents the status of a check
type CheckStatus string

const (
	CheckStatusPass    CheckStatus = "pass"
	CheckStatusFail    CheckStatus = "fail"
	CheckStatusWarning CheckStatus = "warning"
	CheckStatusSkipped CheckStatus = "skipped"
)

// MultiRegistryValidationResult represents validation results for multiple registries
type MultiRegistryValidationResult struct {
	Timestamp   time.Time                            `json:"timestamp"`
	Duration    time.Duration                        `json:"duration"`
	Results     map[string]*RegistryValidationResult `json:"results"`
	HasFailures bool                                 `json:"has_failures"`
}

// RegistryValidationResult represents validation result for a single registry
type RegistryValidationResult struct {
	Registry             string    `json:"registry"`
	Timestamp            time.Time `json:"timestamp"`
	OverallStatus        string    `json:"overall_status"`
	AuthenticationStatus string    `json:"authentication_status"`
	AuthenticationError  string    `json:"authentication_error,omitempty"`
	AuthenticationType   string    `json:"authentication_type,omitempty"`
	Username             string    `json:"username,omitempty"`
	ConnectivityStatus   string    `json:"connectivity_status"`
	ConnectivityError    string    `json:"connectivity_error,omitempty"`
}
