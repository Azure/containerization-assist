package resources

import "context"

// Store defines the interface for resource storage and retrieval.
// This interface is implemented by infrastructure layer.
type Store interface {
	// GetResource retrieves a resource by URI
	GetResource(ctx context.Context, uri string) (Resource, error)

	// ListResources lists all available resources
	ListResources(ctx context.Context) ([]Resource, error)

	// AddResource adds a new resource to the store
	AddResource(ctx context.Context, resource Resource) error

	// RemoveResource removes a resource from the store
	RemoveResource(ctx context.Context, uri string) error

	// RegisterProviders registers resource providers with the MCP server
	RegisterProviders(mcpServer interface{}) error
}

// Resource represents a resource in the system
type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Content     any                    `json:"content,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
