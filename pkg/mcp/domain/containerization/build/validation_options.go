package build

// ValidationOptions contains configuration for validation
type ValidationOptions struct {
	UseHadolint        bool
	Severity           string
	IgnoreRules        []string
	CheckSecrets       bool
	CheckBestPractices bool
	ValidateDockerfile bool
	ValidateContext    bool
	CheckSecurity      bool
	CheckOptimization  bool
	TrustedRegistries  []string
}
