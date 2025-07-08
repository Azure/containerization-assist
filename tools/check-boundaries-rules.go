package main

// BoundaryRules defines the architectural boundaries for pkg/mcp
var boundaryRules = []BoundaryRule{
	{
		Package:     "pkg/mcp/api",
		AllowedDeps: []string{
			// API should only depend on types, no implementations
		},
		ForbiddenDeps: []string{
			"pkg/mcp/core",     // No dependency on implementations
			"pkg/mcp/tools",    // No dependency on tool implementations
			"pkg/mcp/internal", // No internal dependencies
		},
	},
	{
		Package: "pkg/mcp/core",
		AllowedDeps: []string{
			"pkg/mcp/api",     // Core depends on API definitions
			"pkg/mcp/session", // Session management
		},
		ForbiddenDeps: []string{
			"pkg/mcp/tools",    // No direct tool dependencies
			"pkg/mcp/workflow", // Core doesn't depend on workflow
		},
	},
	{
		Package: "pkg/mcp/tools",
		AllowedDeps: []string{
			"pkg/mcp/api",      // Tool interfaces
			"pkg/mcp/session",  // Session access
			"pkg/mcp/security", // Security validation
			"pkg/mcp/internal", // Implementation details
		},
		ForbiddenDeps: []string{
			"pkg/mcp/core",     // Tools don't depend on core
			"pkg/mcp/workflow", // Tools don't depend on workflow
		},
	},
	{
		Package: "pkg/mcp/session",
		AllowedDeps: []string{
			"pkg/mcp/api",      // Session interfaces
			"pkg/mcp/storage",  // Persistence
			"pkg/mcp/internal", // Implementation details
		},
		ForbiddenDeps: []string{
			"pkg/mcp/tools",    // Session doesn't depend on tools
			"pkg/mcp/core",     // Session doesn't depend on core
			"pkg/mcp/workflow", // Session doesn't depend on workflow
		},
	},
	{
		Package: "pkg/mcp/workflow",
		AllowedDeps: []string{
			"pkg/mcp/api",      // Workflow interfaces
			"pkg/mcp/tools",    // Tool orchestration
			"pkg/mcp/session",  // Session management
			"pkg/mcp/internal", // Implementation details
		},
		ForbiddenDeps: []string{
			"pkg/mcp/core", // Workflow uses core through API
		},
	},
	{
		Package: "pkg/mcp/transport",
		AllowedDeps: []string{
			"pkg/mcp/api",      // Transport interfaces
			"pkg/mcp/core",     // Server integration
			"pkg/mcp/internal", // Implementation details
		},
		ForbiddenDeps: []string{
			"pkg/mcp/tools",    // Transport doesn't depend on tools
			"pkg/mcp/workflow", // Transport doesn't depend on workflow
		},
	},
	{
		Package: "pkg/mcp/storage",
		AllowedDeps: []string{
			"pkg/mcp/api",      // Storage interfaces
			"pkg/mcp/internal", // Implementation details
		},
		ForbiddenDeps: []string{
			"pkg/mcp/tools",   // Storage doesn't depend on tools
			"pkg/mcp/core",    // Storage doesn't depend on core
			"pkg/mcp/session", // Session uses storage, not vice versa
		},
	},
	{
		Package: "pkg/mcp/security",
		AllowedDeps: []string{
			"pkg/mcp/api",      // Security interfaces
			"pkg/mcp/internal", // Implementation details
		},
		ForbiddenDeps: []string{
			"pkg/mcp/tools",   // Security doesn't depend on tools
			"pkg/mcp/core",    // Security doesn't depend on core
			"pkg/mcp/session", // Security doesn't depend on session
		},
	},
	{
		Package: "pkg/mcp/templates",
		AllowedDeps: []string{
			"pkg/mcp/api",      // Template interfaces
			"pkg/mcp/internal", // Implementation details
		},
		ForbiddenDeps: []string{
			"pkg/mcp/tools",   // Templates don't depend on tools
			"pkg/mcp/core",    // Templates don't depend on core
			"pkg/mcp/session", // Templates don't depend on session
		},
	},
	{
		Package: "pkg/mcp/internal",
		AllowedDeps: []string{
			// Internal can depend on other internal packages
			"pkg/mcp/internal", // Internal dependencies allowed
		},
		ForbiddenDeps: []string{
			"pkg/mcp/api",      // Internal shouldn't depend on API
			"pkg/mcp/core",     // Internal shouldn't depend on core
			"pkg/mcp/tools",    // Internal shouldn't depend on tools
			"pkg/mcp/session",  // Internal shouldn't depend on session
			"pkg/mcp/workflow", // Internal shouldn't depend on workflow
		},
	},
}

// BoundaryRule defines allowed and forbidden dependencies for a package
type BoundaryRule struct {
	Package       string
	AllowedDeps   []string
	ForbiddenDeps []string
}
