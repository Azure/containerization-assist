package manifests

// ManifestStrategy defines how to generate specific types of manifests
type ManifestStrategy interface {
	// GenerateManifest generates a specific type of manifest
	GenerateManifest(options GenerationOptions, context TemplateContext) ([]byte, error)

	// GetManifestType returns the type of manifest this strategy generates
	GetManifestType() string

	// ValidateOptions validates the options for this specific manifest type
	ValidateOptions(options GenerationOptions) error
}
