package containerization.knowledge_filter

# Test production environment filtering
test_production_environment if {
	result_prod := result with input as {"environment": "production"}

	# Should boost security and reliability
	result_prod.categoryWeights.security == 2.0
	result_prod.categoryWeights.reliability == 1.5

	# Should reduce optimization priority
	result_prod.categoryWeights.optimization == 0.7

	# Should boost distroless and minimal tags
	result_prod.tagWeights.distroless == 2.0
	result_prod.tagWeights.minimal == 1.8

	# Should reduce debug tags
	result_prod.tagWeights.debug == 0.3

	# Should restrict registries
	count(result_prod.allowedRegistries) == 3
	"mcr.microsoft.com" in result_prod.allowedRegistries
}

# Test development environment filtering
test_development_environment if {
	result_dev := result with input as {"environment": "development"}

	# Should boost build and caching
	result_dev.categoryWeights.build == 1.8
	result_dev.categoryWeights.caching == 2.0

	# Should reduce security weight (but not eliminate)
	result_dev.categoryWeights.security == 0.8

	# Should boost debug and hot-reload
	result_dev.tagWeights.debug == 1.5
	result_dev.tagWeights["hot-reload"] == 1.8

	# Should not restrict registries in development
	count(result_dev.allowedRegistries) == 0
}

# Test staging environment filtering
test_staging_environment if {
	result_staging := result with input as {"environment": "staging"}

	# Should have balanced weights
	result_staging.categoryWeights.security == 1.5
	result_staging.categoryWeights.reliability == 1.3

	# Should allow more registries than production
	count(result_staging.allowedRegistries) == 4
	"docker.io" in result_staging.allowedRegistries
}

# Test generate-dockerfile tool with production
test_generate_dockerfile_production if {
	result_gen := result with input as {
		"tool": "generate-dockerfile",
		"environment": "production",
	}

	# Should have very high security priority
	result_gen.categoryWeights.security == 2.5

	# Should strongly prefer multi-stage builds
	result_gen.tagWeights["multi-stage"] == 2.0
	result_gen.tagWeights.distroless == 2.0

	# Should limit snippets
	result_gen.maxSnippets == 8

	# Should only allow specific registries
	count(result_gen.allowedRegistries) == 2
}

# Test fix-dockerfile tool
test_fix_dockerfile if {
	result_fix := result with input as {"tool": "fix-dockerfile"}

	# Should prioritize security issues
	result_fix.categoryWeights.security == 2.0

	# Should highlight anti-patterns and vulnerabilities
	result_fix.tagWeights["anti-pattern"] == 2.0
	result_fix.tagWeights.vulnerability == 2.5
}

# Test Java + production filtering
test_java_production if {
	result_java := result with input as {
		"language": "java",
		"environment": "production",
	}

	# Should strongly prefer distroless for Java
	result_java.tagWeights.distroless == 2.5

	# Should prefer JRE over JDK
	result_java.tagWeights.jre == 1.5
	result_java.tagWeights.jdk == 0.5

	# Should exclude JDK snippets
	count(result_java.excludeSnippets) > 0
}

# Test Node.js + production filtering
test_node_production if {
	result_node := result with input as {
		"language": "node",
		"environment": "production",
	}

	# Should prefer Alpine for Node
	result_node.tagWeights.alpine == 1.5

	# Should prefer LTS versions
	result_node.tagWeights.lts == 1.8
}

# Test Microsoft-only registry policy
test_microsoft_only if {
	result_ms := result with input as {"tags": ["microsoft", "azure"]}

	# Should only allow MCR
	count(result_ms.allowedRegistries) == 1
	result_ms.allowedRegistries[0] == "mcr.microsoft.com"

	# Should only allow official and mariner categories
	count(result_ms.allowedBaseImageCategories) == 2
	"official" in result_ms.allowedBaseImageCategories
	"mariner" in result_ms.allowedBaseImageCategories
}

# Test default filter (no special context)
test_default_filter if {
	result_default := result with input as {}

	# Should return empty filters
	count(result_default.excludeSnippets) == 0
	count(result_default.snippetWeights) == 0
	count(result_default.categoryWeights) == 0
	count(result_default.tagWeights) == 0
}
