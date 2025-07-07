# ADR-002: Go Embed for YAML Templates

Date: 2025-07-07
Status: Accepted
Context: Large YAML strings embedded in Go code create cognitive load and maintenance issues
Decision: Use go:embed with /resources/*.yaml for all template content
Consequences:
- Easier: Template editing, version control diffs, external tooling
- Harder: Requires Go 1.16+, slight runtime complexity
