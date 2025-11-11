package containerization.base_images

# ==============================================================================
# Base Image Governance Policy
# ==============================================================================
#
# This policy enforces base image restrictions and recommendations.
# Ensures teams use approved, secure, and efficient base images.
#
# Policy enforcement happens at multiple points:
# - generate-dockerfile: Validates generated Dockerfile plans
# - fix-dockerfile: Validates actual Dockerfile content
# - generate-k8s-manifests: Validates K8s manifest plans (base image references)
#
# ==============================================================================

# Metadata
policy_name := "Base Image Governance"

policy_version := "2.0"

policy_category := "quality"

# Default enforcement level
default enforcement := "advisory"

# ==============================================================================
# INPUT TYPE DETECTION
# ==============================================================================

# Detect if input is a Dockerfile
is_dockerfile if {
	contains(input.content, "FROM ")
}

# ==============================================================================
# BASE IMAGE RULES
# ==============================================================================

# Rule: require-microsoft-images (priority: 95)
# Require Microsoft Container Registry images for all deployments
violations contains result if {
	is_dockerfile
	# Match any FROM line
	regex.match(`(?im)FROM\s+[a-z0-9._/-]+:`, input.content)
	# But NOT mcr.microsoft.com
	not regex.match(`(?im)FROM\s+mcr\.microsoft\.com/`, input.content)

	result := {
		"rule": "require-microsoft-images",
		"category": "quality",
		"priority": 95,
		"severity": "block",
		"message": "Only Microsoft Container Registry images are allowed. Use mcr.microsoft.com/openjdk/jdk with -azurelinux tags for Java, mcr.microsoft.com/dotnet for .NET, mcr.microsoft.com/azurelinux/base for language runtimes.",
		"description": "Require Microsoft Container Registry images for all deployments",
	}
}

# Rule: recommend-microsoft-images (priority: 85)
# Recommend Microsoft Azure Linux images for enterprise deployments
suggestions contains result if {
	is_dockerfile
	regex.match(`(?im)FROM\s+mcr\.microsoft\.com/(openjdk|dotnet|azurelinux)`, input.content)

	result := {
		"rule": "recommend-microsoft-images",
		"category": "quality",
		"priority": 85,
		"severity": "suggest",
		"message": "Good choice using Microsoft Azure Linux 3.0 base images for enterprise deployments. Provides enterprise support and security.",
		"description": "Recommend Microsoft Azure Linux images for enterprise deployments",
	}
}

# Rule: block-latest-tag (priority: 80)
# Prevent use of :latest tag for reproducibility
violations contains result if {
	is_dockerfile
	regex.match(`(?im)FROM\s+[^:]+:latest`, input.content)

	result := {
		"rule": "block-latest-tag",
		"category": "quality",
		"priority": 80,
		"severity": "block",
		"message": "Using :latest tag is not allowed. Specify explicit version tags for reproducibility.",
		"description": "Prevent use of :latest tag for reproducibility",
	}
}

# Rule: recommend-alpine (priority: 60)
# Recommend Alpine variants for smaller images
warnings contains result if {
	is_dockerfile
	regex.match(`(?im)FROM\s+(node|python|ruby):(?!.*alpine)`, input.content)

	result := {
		"rule": "recommend-alpine",
		"category": "performance",
		"priority": 60,
		"severity": "warn",
		"message": "Consider using Alpine variant for smaller image size (e.g., node:20-alpine).",
		"description": "Recommend Alpine variants for smaller images",
	}
}

# Rule: recommend-distroless (priority: 70)
# Recommend distroless images for production
suggestions contains result if {
	is_dockerfile
	regex.match(`(?im)FROM\s+(java|openjdk|golang|go):(?!.*gcr\.io/distroless)`, input.content)

	result := {
		"rule": "recommend-distroless",
		"category": "security",
		"priority": 70,
		"severity": "suggest",
		"message": "Consider using distroless images for enhanced security (e.g., gcr.io/distroless/java).",
		"description": "Recommend distroless images for production",
	}
}

# Rule: block-deprecated-node (priority: 90)
# Block deprecated Node.js versions
violations contains result if {
	is_dockerfile
	regex.match(`(?im)FROM\s+node:(8|10|12|14|16)\b`, input.content)

	result := {
		"rule": "block-deprecated-node",
		"category": "quality",
		"priority": 90,
		"severity": "block",
		"message": "Deprecated Node.js version detected. Use Node.js 18 or higher.",
		"description": "Block deprecated Node.js versions",
	}
}

# Rule: block-deprecated-python (priority: 90)
# Block deprecated Python versions
violations contains result if {
	is_dockerfile
	regex.match(`(?im)FROM\s+python:(2\.7|3\.[0-6])\b`, input.content)

	result := {
		"rule": "block-deprecated-python",
		"category": "quality",
		"priority": 90,
		"severity": "block",
		"message": "Deprecated Python version detected. Use Python 3.10 or higher.",
		"description": "Block deprecated Python versions",
	}
}

# Rule: block-oversized-base (priority: 65)
# Warn about large base images
warnings contains result if {
	is_dockerfile
	regex.match(`(?im)FROM\s+(ubuntu|centos|fedora):(?!.*minimal)`, input.content)

	result := {
		"rule": "block-oversized-base",
		"category": "performance",
		"priority": 65,
		"severity": "warn",
		"message": "Large base images detected. Consider Alpine, slim, or distroless variants.",
		"description": "Warn about large base images",
	}
}

# Rule: recommend-specific-versions (priority: 75)
# Recommend specific version tags
suggestions contains result if {
	is_dockerfile
	regex.match(`(?im)FROM\s+[^:@]+:(\d+)$`, input.content)

	result := {
		"rule": "recommend-specific-versions",
		"category": "quality",
		"priority": 75,
		"severity": "suggest",
		"message": "Consider using more specific version tags (e.g., 20.11-alpine instead of 20).",
		"description": "Recommend specific version tags",
	}
}

# ==============================================================================
# POLICY DECISION
# ==============================================================================

# Allow if no blocking violations
default allow := false

allow if {
	count(violations) == 0
}

# Final result structure
result := {
	"allow": allow,
	"violations": violations,
	"warnings": warnings,
	"suggestions": suggestions,
	"summary": {
		"total_violations": count(violations),
		"total_warnings": count(warnings),
		"total_suggestions": count(suggestions),
	},
}
