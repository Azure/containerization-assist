# ADR-004: Policy-Based Configuration System

**Date:** 2025-10-17
**Status:** Accepted
**Deciders:** Engineering Team
**Context:** We needed a flexible, declarative way to enforce organizational standards for container security, quality, and compliance that could be version-controlled, environment-specific, and modified without code changes.

## Decision

We decided to implement a YAML-based policy system with a rule evaluation engine. Policies are defined in `policies/*.yaml` files using a structured format with matchers (regex/function), conditions, and actions (block/warn/suggest).

**Implementation:**

```yaml
# policies/security-baseline.yaml
version: '2.0'
metadata:
  name: Security Baseline
  description: Essential security rules
  category: security

defaults:
  enforcement: strict  # strict | advisory | lenient
  security:
    scanners:
      required: true
      tools: ['trivy']
    nonRootUser: true

rules:
  - id: block-root-user
    category: security
    priority: 95
    description: Detect and block root user in Dockerfiles
    conditions:
      - kind: regex
        pattern: '^USER\s+(root|0)\s*$'
        flags: m
    actions:
      block: true
      message: 'Running as root user is not allowed. Add USER directive with non-root user.'
```

**Architecture:**

```typescript
// Modular policy system (5 modules)
@config/policy-schemas.ts   // Zod schemas and TypeScript types
@config/policy-io.ts        // Load, validate, cache
@config/policy-eval.ts      // Rule evaluation engine
@config/policy-prompt.ts    // AI prompt constraint integration
@config/policy-constraints.ts // Data-driven constraint extraction
```

## Rationale

1. **Declarative Configuration:** YAML policies separate business rules from code logic, enabling non-developers to modify policies
2. **Version Control:** Policy files are committed to git, providing audit trail and change history
3. **Environment-Specific:** Different policy files for dev/staging/production with targeted enforcement
4. **Composability:** Multiple policy files can be merged, with later files overriding earlier ones
5. **Type Safety:** Zod schemas validate policy structure at load time with actionable error messages
6. **Flexibility:** Supports both regex patterns and function matchers for complex validation logic
7. **AI Integration:** Policies can inject constraints into AI prompts for Dockerfile generation
8. **No Runtime Dependencies:** Pure TypeScript evaluation with no external rule engines

## Consequences

### Positive

- **Organizational Governance:** Centralized security and quality standards across teams
- **Zero Code Changes:** Policy updates don't require rebuilding or redeploying the application
- **Multiple Environments:** Different enforcement levels (strict/advisory/lenient) per environment
- **Rich Matcher System:** Regex patterns + 4 function matchers (hasPattern, fileExists, largerThan, hasVulnerabilities)
- **Actionable Feedback:** Block/warn/suggest actions with contextual messages guide users
- **Type-Safe Validation:** Zod schemas catch invalid policies at load time
- **Caching:** Single-load caching with simple invalidation for CLI performance
- **3 Production Policies:** security-baseline.yaml, container-best-practices.yaml, base-images.yaml
- **Testability:** Pure functions in policy-eval.ts make testing straightforward
- **Priority System:** Rules execute in priority order (security: 90-100, quality: 70-89, performance: 50-69)

### Negative

- **YAML Complexity:** Non-technical users may find YAML indentation challenging
- **No GUI Editor:** Policies must be edited manually in text editor
- **Limited Matcher Functions:** Only 4 built-in function matchers (extensible but requires code)
- **Regex Learning Curve:** Writing effective regex patterns requires expertise
- **Validation at Load Time Only:** Invalid policies fail at startup, not authoring time
- **No Policy Versioning:** Policies are version '2.0' but no migration path from 1.x
- **Static Evaluation:** Cannot evaluate dynamic runtime conditions (e.g., network policies)

## Alternatives Considered

### Alternative 1: Hardcoded Rules in TypeScript

- **Pros:**
  - Full TypeScript type safety
  - IDE autocomplete and refactoring
  - No parsing overhead
  - Easier to write complex logic
- **Cons:**
  - Requires code changes for policy updates
  - Need rebuild and redeploy for rule changes
  - No separation of concerns (rules mixed with code)
  - Difficult for non-developers to modify
- **Rejected because:** Doesn't scale for organizations with multiple teams and environments

### Alternative 2: JSON Configuration

- **Pros:**
  - Structured format with JSON Schema validation
  - Native JavaScript parsing (no dependencies)
  - Easier to generate programmatically
- **Cons:**
  - Less human-readable than YAML
  - No comments support
  - More verbose for complex nested structures
  - Still requires the same evaluation engine
- **Rejected because:** YAML's readability and comment support better serve policy documentation needs

### Alternative 3: Open Policy Agent (OPA) with Rego

- **Pros:**
  - Industry-standard policy engine
  - Rich query language (Rego)
  - Battle-tested at scale
  - Built-in policy distribution
- **Cons:**
  - Additional runtime dependency (OPA binary)
  - Steep learning curve for Rego language
  - Overkill for single-operator CLI tool
  - Increased complexity for simple regex rules
  - External process communication overhead
- **Rejected because:** Too heavyweight for a single-user containerization assistant

### Alternative 4: Database-Driven Configuration

- **Pros:**
  - Dynamic updates without file system changes
  - Potential for web UI editor
  - Centralized management
  - Role-based access control
- **Cons:**
  - Requires database setup and maintenance
  - Network dependency for policy loading
  - Complex deployment story
  - Version control requires additional tooling
  - Offline usage not supported
- **Rejected because:** Adds operational complexity for a CLI tool designed for local development

## Related Decisions

- **ADR-001: Result<T> Error Handling Pattern** - All policy functions return Result<T> for consistent error handling
- **ADR-003: Knowledge Enhancement System** - Policies provide deterministic constraints while knowledge packs provide AI guidance
- **ADR-006: Infrastructure Layer Organization** - Policies organized in @config layer with modular architecture

## References

- **Policy Schemas:** `src/config/policy-schemas.ts` - Zod schemas and TypeScript types
- **Policy I/O:** `src/config/policy-io.ts` - Load, validate, cache operations (97 lines)
- **Policy Evaluation:** `src/config/policy-eval.ts` - Rule evaluation engine with matcher system
- **Policy Files:** `policies/security-baseline.yaml`, `policies/container-best-practices.yaml`, `policies/base-images.yaml`
- **Test Coverage:** `test/unit/config/policy-*.test.ts` (6 comprehensive test files)
