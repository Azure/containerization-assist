package containerization.knowledge_filter

# Knowledge Filter Policy Example
#
# This policy demonstrates how to control knowledge snippet filtering and weighting
# based on environment, tool context, and organizational preferences.
#
# Features demonstrated:
# - Environment-based filtering (dev vs production)
# - Category weight adjustments (prioritize security in prod)
# - Tag-based weight boosting
# - Base image registry filtering
# - Snippet exclusion
#
# Rules are evaluated in order of specificity (most specific first)

# Default filter (no restrictions)
default result := {
	"excludeSnippets": [],
	"snippetWeights": {},
	"categoryWeights": {},
	"tagWeights": {},
}

# Tool-specific filtering: generate-dockerfile + production (most specific)
result := {
	"categoryWeights": {
		"security": 2.5, # Even higher security priority during generation
		"build": 1.5,
		"optimization": 1.2,
	},
	"tagWeights": {
		"multi-stage": 2.0, # Strongly prefer multi-stage builds
		"distroless": 2.0,
		"minimal": 1.8,
		"layer-caching": 1.5,
	},
	"allowedRegistries": [
		"mcr.microsoft.com",
		"gcr.io",
	],
	"allowedBaseImageCategories": [
		"official",
		"distroless",
	],
	"maxSnippets": 8,
} if {
	input.tool == "generate-dockerfile"
	input.environment == "production"
}

# Language-specific filtering: Java + production
result := {
	"categoryWeights": {
		"security": 2.0,
		"optimization": 1.5,
	},
	"tagWeights": {
		"distroless": 2.5, # Java works great with distroless
		"jre": 1.5, # Prefer JRE over JDK in production
		"jdk": 0.5, # Reduce JDK recommendations
	},
	"allowedRegistries": [
		"mcr.microsoft.com",
		"gcr.io",
	],
	"excludeSnippets": [
		"java-openjdk-full",
		"java-dev-tools",
	],
} if {
	input.language == "java"
	input.environment == "production"
	not input.tool == "generate-dockerfile" # Avoid conflict with generate-dockerfile rule
}

# Language-specific filtering: Node.js + production
result := {
	"categoryWeights": {
		"security": 2.0,
		"optimization": 1.5,
	},
	"tagWeights": {
		"alpine": 1.5, # Alpine is great for Node.js
		"distroless": 1.3,
		"lts": 1.8, # Prefer LTS versions
	},
	"allowedRegistries": [
		"mcr.microsoft.com",
		"gcr.io",
	],
} if {
	input.language == "node"
	input.environment == "production"
	not input.tool == "generate-dockerfile" # Avoid conflict
}

# Production environment: prioritize security and reliability
result := {
	"categoryWeights": {
		"security": 2.0,
		"reliability": 1.5,
		"resilience": 1.5,
		"optimization": 0.7,
	},
	"tagWeights": {
		"distroless": 2.0,
		"minimal": 1.8,
		"hardening": 1.5,
		"non-root": 1.5,
		"read-only": 1.3,
		"debug": 0.3,
		"hot-reload": 0.2,
	},
	"allowedRegistries": [
		"mcr.microsoft.com",
		"gcr.io",
		"registry.gitlab.com",
	],
	"allowedBaseImageCategories": [
		"official",
		"distroless",
		"security",
	],
	"excludeSnippets": [
		"dockerfile-dev-hot-reload",
		"dockerfile-debug-tools",
		"dockerfile-dev-dependencies",
	],
	"minScore": 15,
} if {
	input.environment == "production"
	not input.tool == "generate-dockerfile" # More specific rule exists
	not input.language # No language-specific rule
}

# Development environment: prioritize speed and debugging
result := {
	"categoryWeights": {
		"build": 1.8,
		"caching": 2.0,
		"security": 0.8,
	},
	"tagWeights": {
		"single-stage": 1.5,
		"cache": 1.8,
		"debug": 1.5,
		"hot-reload": 1.8,
		"distroless": 0.5,
		"minimal": 0.6,
		"multi-stage": 0.8,
	},
	"allowedRegistries": [],
	"allowedBaseImageCategories": [],
	"excludeSnippets": [
		"security-read-only-rootfs",
		"security-drop-all-capabilities",
	],
} if {
	input.environment == "development"
}

# Staging environment: balanced approach
result := {
	"categoryWeights": {
		"security": 1.5,
		"reliability": 1.3,
		"optimization": 1.0,
	},
	"tagWeights": {
		"distroless": 1.3,
		"minimal": 1.2,
		"debug": 0.8,
	},
	"allowedRegistries": [
		"mcr.microsoft.com",
		"gcr.io",
		"registry.gitlab.com",
		"docker.io",
	],
} if {
	input.environment == "staging"
}

# Tool-specific filtering: fix-dockerfile
result := {
	"categoryWeights": {
		"security": 2.0,
		"optimization": 1.5,
		"build": 1.0,
	},
	"tagWeights": {
		"anti-pattern": 2.0,
		"vulnerability": 2.5,
		"best-practice": 1.5,
	},
} if {
	input.tool == "fix-dockerfile"
	not input.environment # No environment-specific rule
}

# Microsoft-only registry policy (corporate compliance)
result := {
	"allowedRegistries": [
		"mcr.microsoft.com",
	],
	"allowedBaseImageCategories": [
		"official",
		"mariner",
	],
	"categoryWeights": {
		"security": 2.0,
	},
} if {
	input.tags
	"microsoft" in input.tags
}
