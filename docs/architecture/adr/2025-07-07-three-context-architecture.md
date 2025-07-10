# ADR-001: Three-Context Architecture Model

Date: 2025-07-07
Status: Accepted
Context: Current 30+ package structure has deep nesting, import cycles, and unclear boundaries
Decision: Adopt 3-bounded-context architecture: domain/, application/, infra/
Consequences:
- Easier: Clear separation of concerns, reduced import cycles, better testability
- Harder: Initial migration effort, need to understand bounded context principles
