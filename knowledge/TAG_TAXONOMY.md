# Knowledge Pack Tag Taxonomy

**Version:** 1.0.0
**Last Updated:** 2025-10-21
**Status:** Active

## Overview

This document defines the canonical tag taxonomy for knowledge pack entries in the containerization-assist MCP server. Tags enable precise knowledge matching based on tool context, language, vendor, and other dimensions.

## Tag Categories

### Tool Tags

Used to filter knowledge by MCP tool context. These tags ensure that knowledge entries are surfaced in the most relevant tool invocations.

- `generate-dockerfile` - Dockerfile generation guidance
- `fix-dockerfile` - Dockerfile validation and fixing recommendations
- `scan-image` - Security scanning context and remediation
- `verify-deploy` - Deployment verification checks
- `deploy` - Deployment execution best practices
- `generate-k8s-manifests` - Kubernetes manifest generation
- `prepare-cluster` - Cluster preparation and prerequisites

**Usage:** Add tool tags to entries that are specifically relevant to certain tools. An entry can have multiple tool tags if it applies to multiple contexts.

### Vendor Tags

Identify vendor/provider-specific recommendations. Use these for cloud platform-specific guidance, proprietary base images, or vendor tooling.

- `microsoft` - Microsoft-related technologies (general)
- `azure` - Azure-specific recommendations (more specific than `microsoft`)
- `google` - Google-related technologies (general)
- `gcp` - Google Cloud Platform-specific (alias for `google`)
- `aws` - Amazon Web Services
- `redhat` - Red Hat / OpenShift
- `canonical` - Canonical / Ubuntu

**Vendor Tag Aliases:**
- `mcr`, `msft` → `microsoft`
- `gcp`, `gcr` → `google`
- `eks`, `ecr` → `aws`
- `aks`, `acr` → `azure`

**Usage:** Tag vendor-specific recommendations to help users in enterprise contexts find relevant guidance.

### Language Tags (Canonical Forms)

Use canonical language names. The alias system handles common variations automatically.

**Canonical Tags:**
- `node` (for JavaScript/TypeScript/Node.js)
- `python`
- `java`
- `dotnet` (for C#/.NET)
- `go`
- `ruby`
- `php`
- `rust`

**Language Aliases:**
- `javascript`, `typescript`, `nodejs`, `js` → `node`
- `py` → `python`
- `csharp` → `dotnet`
- `golang` → `go`

**Usage:** Only use the canonical form in tags. Do NOT add both `node` and `javascript` to the same entry - the alias system handles this automatically.

### Build Tool Tags

Specific build tools and package managers. Use these to surface build-specific knowledge.

- `maven` - Apache Maven
- `maven-wrapper` - Maven Wrapper (more specific than `maven`)
- `gradle` - Gradle
- `gradle-wrapper` - Gradle Wrapper (more specific than `gradle`)
- `npm-ci` - npm clean install
- `pip-install` - Python pip
- `go-mod` - Go modules
- `cargo-build` - Rust cargo
- `build-tool` - Generic build tool tag (use with specific tags)

**Build Tool Aliases:**
- `mvn` → `maven`
- `gradlew` → `gradle`
- `cargo` → `rust`

**Usage:** Be specific when possible (e.g., `maven-wrapper` instead of just `wrapper`). Include the generic `build-tool` tag for broad build-related queries.

### Stage Tags

Multi-stage build stage indicators. Use these to clarify which stage of a multi-stage Dockerfile a recommendation applies to.

- `build-stage` - Build/compilation stage recommendations
- `runtime-stage` - Final runtime stage recommendations
- `intermediate-stage` - Intermediate stages (rare)

**Usage:** Add stage tags to entries that are specific to certain stages of multi-stage builds. An entry can have multiple stage tags if it applies to multiple stages.

### Category Tags

Generic categorization tags. Use sparingly - prefer more specific tags when possible.

- `security` - Security-related recommendations
- `optimization` - Performance and size optimizations
- `performance` - Runtime performance
- `monitoring` - Observability and monitoring
- `networking` - Network configuration
- `multistage` - Multi-stage build patterns
- `production` - Production environment
- `development` - Development environment
- `alpine` - Alpine Linux
- `distroless` - Distroless images
- `minimal` - Minimal/slim images (alias for `distroless`)

**Usage:** Use these for broad categorization, but combine with more specific tags for better matching.

## Tagging Guidelines

### 1. Use Canonical Forms

Always use canonical language and vendor names. The alias system handles variations.

**Good:**
```json
{
  "tags": ["node", "alpine", "production"]
}
```

**Bad (redundant):**
```json
{
  "tags": ["node", "javascript", "typescript", "alpine", "production"]
}
```

### 2. Be Specific

Prefer specific tags over generic ones.

**Good:**
```json
{
  "tags": ["maven-wrapper", "build-tool", "java", "build-stage"]
}
```

**Bad (too generic):**
```json
{
  "tags": ["wrapper", "build", "java"]
}
```

### 3. Tag for Tools

Always add relevant tool tags to help knowledge surface in the right context.

**Good:**
```json
{
  "id": "dockerfile-user-root",
  "tags": ["security", "user", "root", "fix-dockerfile", "generate-dockerfile", "scan-image"]
}
```

**Bad (missing tool context):**
```json
{
  "id": "dockerfile-user-root",
  "tags": ["security", "user", "root"]
}
```

### 4. Include Vendor When Relevant

Tag vendor-specific recommendations explicitly.

**Good:**
```json
{
  "id": "distroless-security",
  "tags": ["distroless", "security", "production", "google", "runtime-stage"]
}
```

**Bad (missing vendor):**
```json
{
  "id": "distroless-security",
  "tags": ["distroless", "security", "production"]
}
```

### 5. Indicate Stage for Multi-Stage Builds

Tag multi-stage build recommendations with stage indicators.

**Good:**
```json
{
  "id": "dotnet-sdk-multistage",
  "tags": ["dotnet", "multistage", "optimization", "build-stage", "runtime-stage"]
}
```

**Bad (missing stage context):**
```json
{
  "id": "dotnet-sdk-multistage",
  "tags": ["dotnet", "multistage", "optimization"]
}
```

### 6. Avoid Redundancy

Don't add tags that are covered by aliases or are redundant.

**Good:**
```json
{
  "tags": ["node", "alpine"]
}
```

**Bad (redundant due to aliases):**
```json
{
  "tags": ["node", "nodejs", "javascript", "alpine"]
}
```

**Exception:** Keep both when frameworks differ (e.g., Django vs. Flask both use Python but have different recommendations).

## Tag Matching and Scoring

The knowledge matcher uses weighted scoring for tag matches:

- **TOOL** (25 points) - High weight for tool-specific matches
- **CATEGORY** (20 points) - Category matches
- **VENDOR** (12 points) - Medium weight for vendor matches
- **LANGUAGE** (15 points) - Language matches
- **TAG** (10 points) - Generic tag matches
- **FRAMEWORK** (10 points) - Framework matches
- **ENVIRONMENT** (8 points) - Environment matches

Additionally, severity affects scoring:
- `required` - +50 points
- `high` - +15 points
- `medium` - +10 points
- `low` - +5 points

## Common Patterns

### Security Entries

```json
{
  "id": "dockerfile-user-root",
  "category": "security",
  "severity": "high",
  "tags": ["security", "user", "root", "fix-dockerfile", "generate-dockerfile", "scan-image"]
}
```

### Vendor-Specific Base Images

```json
{
  "id": "distroless-security",
  "category": "dockerfile",
  "severity": "high",
  "tags": ["distroless", "security", "production", "google", "runtime-stage", "generate-dockerfile"]
}
```

### Language-Specific Build Tools

```json
{
  "id": "maven-wrapper-setup",
  "category": "dockerfile",
  "severity": "high",
  "tags": ["maven", "maven-wrapper", "build-tool", "reproducible", "java", "build-stage", "generate-dockerfile"]
}
```

### Multi-Stage Build Patterns

```json
{
  "id": "node-alpine-multistage",
  "category": "dockerfile",
  "severity": "high",
  "tags": ["node", "alpine", "multistage", "optimization", "build-stage", "runtime-stage", "generate-dockerfile"]
}
```

### Kubernetes Deployment

```json
{
  "id": "k8s-health-checks",
  "category": "kubernetes",
  "severity": "required",
  "tags": ["health", "liveness", "readiness", "deploy", "verify-deploy", "generate-k8s-manifests"]
}
```

## Version History

### v1.0.0 (2025-10-21)
- Initial tag taxonomy
- Added tool tags (7 tools)
- Added vendor tags (6 vendors)
- Added build tool tags
- Added stage tags for multi-stage builds
- Defined canonical language forms
- Established tagging guidelines

## Related Documentation

- **Matcher Implementation:** `src/knowledge/matcher.ts`
- **Knowledge Schema:** `src/knowledge/schemas.ts`
- **Knowledge Types:** `src/knowledge/types.ts`
- **Sample Packs:** `knowledge/packs/`
- **Project Guidelines:** `CLAUDE.md`

## Contributing

When adding new tags:

1. Check if an existing tag or alias covers your use case
2. Prefer extending aliases over creating new canonical tags
3. Add new tags to the appropriate category in this document
4. Update the version history
5. Update any affected knowledge pack entries
6. Add tests for new tag matching behavior

## Questions?

For questions about tag usage or suggestions for new tags, please refer to the project's contribution guidelines or open an issue.
