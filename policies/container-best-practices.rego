package containerization.best_practices

# ==============================================================================
# Container Best Practices Policy
# ==============================================================================
#
# This policy enforces Docker and container best practices for production
# readiness, including health checks, layer optimization, and security.
#
# Policy enforcement happens at multiple points:
# - generate-dockerfile: Validates generated Dockerfile plans
# - fix-dockerfile: Validates actual Dockerfile content
#
# ==============================================================================

# Metadata
policy_name := "Container Best Practices"

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
# BEST PRACTICE RULES
# ==============================================================================

# Rule: require-healthcheck (priority: 75)
# Recommend HEALTHCHECK for production containers
warnings contains result if {
	is_dockerfile
	not regex.match(`(?im)^HEALTHCHECK`, input.content)

	result := {
		"rule": "require-healthcheck",
		"category": "quality",
		"priority": 75,
		"severity": "warn",
		"message": "HEALTHCHECK directive is recommended for production containers.",
		"description": "Recommend HEALTHCHECK for production containers",
	}
}

# Rule: require-workdir (priority: 60)
# Recommend WORKDIR instead of cd commands
warnings contains result if {
	is_dockerfile
	regex.match(`(?im)RUN\s+cd\s+/`, input.content)

	result := {
		"rule": "require-workdir",
		"category": "quality",
		"priority": 60,
		"severity": "warn",
		"message": "Use WORKDIR directive instead of cd commands for better readability.",
		"description": "Recommend WORKDIR instead of cd commands",
	}
}

# Rule: avoid-apt-upgrade (priority: 65)
# Avoid apt-get upgrade in Dockerfile
warnings contains result if {
	is_dockerfile
	regex.match(`(?im)apt-get\s+(upgrade|dist-upgrade)`, input.content)

	result := {
		"rule": "avoid-apt-upgrade",
		"category": "quality",
		"priority": 65,
		"severity": "warn",
		"message": "Avoid apt-get upgrade in Dockerfile. Use specific package versions instead.",
		"description": "Avoid apt-get upgrade in Dockerfile",
	}
}

# Rule: excessive-run-commands (priority: 55)
# Detect excessive RUN commands (6 or more)
suggestions contains result if {
	is_dockerfile

	# Count RUN commands
	run_count := count([match |
		regex.find_all_string_submatch_n(`(?im)^RUN\s+`, input.content, -1)[match]
	])

	run_count >= 6

	result := {
		"rule": "excessive-run-commands",
		"category": "performance",
		"priority": 55,
		"severity": "suggest",
		"message": "Consider combining RUN commands with && to reduce layers and image size.",
		"description": "Detect excessive RUN commands",
	}
}

# Rule: apt-cleanup-missing (priority: 60)
# Recommend apt cache cleanup
suggestions contains result if {
	is_dockerfile
	regex.match(`(?im)apt-get\s+install`, input.content)

	result := {
		"rule": "apt-cleanup-missing",
		"category": "performance",
		"priority": 60,
		"severity": "suggest",
		"message": "Clean apt cache after installation: RUN apt-get update && apt-get install -y <packages> && rm -rf /var/lib/apt/lists/*",
		"description": "Recommend apt cache cleanup",
	}
}

# Rule: avoid-sudo (priority: 80)
# Avoid using sudo in containers
warnings contains result if {
	is_dockerfile
	regex.match(`(?im)\bsudo\b`, input.content)

	result := {
		"rule": "avoid-sudo",
		"category": "security",
		"priority": 80,
		"severity": "warn",
		"message": "Avoid using sudo in containers. Run as non-root user instead.",
		"description": "Avoid using sudo in containers",
	}
}

# Rule: recommend-multistage (priority: 65)
# Recommend multi-stage builds for compiled languages
suggestions contains result if {
	is_dockerfile
	regex.match(`(?im)(mvn|gradle|npm\s+run\s+build|go\s+build|cargo\s+build)`, input.content)

	result := {
		"rule": "recommend-multistage",
		"category": "performance",
		"priority": 65,
		"severity": "suggest",
		"message": "Consider using multi-stage builds to reduce final image size.",
		"description": "Recommend multi-stage builds for compiled languages",
	}
}

# Rule: recommend-expose (priority: 45)
# Recommend EXPOSE directive
suggestions contains result if {
	is_dockerfile
	regex.match(`(?im)(PORT|LISTEN|:808|:300|:500)`, input.content)

	result := {
		"rule": "recommend-expose",
		"category": "quality",
		"priority": 45,
		"severity": "suggest",
		"message": "Use EXPOSE directive to document which ports your application listens on.",
		"description": "Recommend EXPOSE directive",
	}
}

# Rule: recommend-npm-ci (priority: 70)
# Recommend npm ci over npm install
suggestions contains result if {
	is_dockerfile
	regex.match(`(?im)npm\s+install(?!\s+-g)`, input.content)

	result := {
		"rule": "recommend-npm-ci",
		"category": "quality",
		"priority": 70,
		"severity": "suggest",
		"message": "Use \"npm ci\" instead of \"npm install\" for reproducible builds.",
		"description": "Recommend npm ci over npm install",
	}
}

# Rule: apk-no-cache (priority: 60)
# Recommend --no-cache with apk add
suggestions contains result if {
	is_dockerfile
	regex.match(`(?im)apk\s+add(?!\s+--no-cache)`, input.content)

	result := {
		"rule": "apk-no-cache",
		"category": "performance",
		"priority": 60,
		"severity": "suggest",
		"message": "Use apk add --no-cache to avoid caching package indexes.",
		"description": "Recommend --no-cache with apk add",
	}
}

# ==============================================================================
# POLICY DECISION
# ==============================================================================

# Best practices policy has no blocking violations (only warnings and suggestions)
violations := set()

# Allow by default (best practices don't block)
default allow := true

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
