# ADR-003: Knowledge Enhancement System

**Date:** 2025-10-17
**Status:** Accepted
**Deciders:** Engineering Team
**Context:** AI-generated Dockerfiles and Kubernetes manifests were inconsistent and often contained hallucinated or outdated best practices. We needed a way to ensure deterministic, high-quality outputs that incorporate real-world knowledge and battle-tested patterns.

## Decision

We decided to enhance AI outputs with static knowledge packs—curated collections of framework-specific configurations, base images, best practices, and proven patterns stored as JSON data.

**Implementation:**

```typescript
// Knowledge pack structure
interface KnowledgePack {
  framework: string;
  baseImages: Array<{
    image: string;
    tag: string;
    variants: string[];
    useCases: string[];
  }>;
  buildPatterns: {
    dependencies: string[];
    buildCommands: string[];
    environment: Record<string, string>;
  };
  bestPractices: {
    multiStage: boolean;
    healthCheck: string;
    securityHardening: string[];
  };
}

// Knowledge enhancement flow
async function generateDockerfile(ctx: GenerationContext): Promise<string> {
  // 1. Detect framework
  const framework = await detectFramework(ctx.repoPath);

  // 2. Load knowledge pack
  const knowledge = await loadKnowledgePack(framework);

  // 3. Enhance AI prompt with knowledge
  const enhancedPrompt = applyKnowledgeConstraints(basePrompt, knowledge);

  // 4. Generate with enhanced context
  return await generateWithKnowledge(enhancedPrompt);
}
```

**Knowledge Organization:**

```
knowledge/packs/
├── languages/
│   ├── node.json         # Node.js base knowledge
│   ├── python.json       # Python base knowledge
│   └── go.json          # Go base knowledge
├── frameworks/
│   ├── express.json      # Express.js patterns
│   ├── fastapi.json     # FastAPI patterns
│   └── nextjs.json      # Next.js patterns
└── platforms/
    ├── docker.json       # Docker best practices
    └── kubernetes.json   # K8s patterns
```

## Rationale

1. **Deterministic Outputs:** Knowledge packs ensure consistent, repeatable results
2. **No Hallucination:** Real configurations from production systems, not AI guesses
3. **Best Practices Embedded:** Curated patterns from experienced practitioners
4. **Framework-Specific:** Tailored knowledge for each technology stack
5. **Maintainable:** Static JSON files can be version controlled and reviewed
6. **Composable:** Multiple knowledge packs can be combined for complex scenarios

## Consequences

### Positive

- **633 entries across 28 packs:** Comprehensive coverage of common frameworks
- **High-quality generation:** Dockerfiles follow production-grade patterns
- **Consistent outputs:** Same input always produces same result
- **No outdated practices:** Knowledge is curated and validated
- **Fast loading:** Static JSON loads in milliseconds
- **Easy to extend:** Adding new frameworks is just adding new JSON files
- **Testable:** Knowledge packs can be unit tested independently

### Negative

- **Maintenance overhead:** Knowledge packs must be kept up-to-date
- **Storage cost:** 28 JSON files add to repository size (minimal impact)
- **Update latency:** New best practices require manual knowledge pack updates
- **Rigid patterns:** May not capture every edge case or custom setup
- **Knowledge drift:** External world changes faster than we can update packs

## Alternatives Considered

### Alternative 1: Pure AI Generation

- **Pros:**
  - No knowledge maintenance required
  - Adapts to any framework automatically
  - Leverages full AI capabilities
- **Cons:**
  - Non-deterministic outputs
  - Hallucination risk
  - May use outdated patterns
  - No guarantees of quality
- **Rejected because:** Production systems require deterministic, reliable outputs

### Alternative 2: Manual Templates

- **Pros:**
  - Complete control over output
  - Fully deterministic
  - No AI needed
- **Cons:**
  - No flexibility for unique projects
  - Requires manual template per framework
  - Can't adapt to repository specifics
  - User must choose correct template
- **Rejected because:** Too rigid; doesn't leverage AI's ability to adapt to specific codebases

### Alternative 3: Dynamic Web Scraping

- **Pros:**
  - Always up-to-date
  - No manual curation
  - Comprehensive coverage
- **Cons:**
  - Non-deterministic
  - Network dependency
  - Quality varies
  - Legal/ethical concerns
  - Slow (network latency)
- **Rejected because:** Unreliable and adds runtime dependencies

### Alternative 4: Hybrid Retrieval-Augmented Generation (RAG)

- **Pros:**
  - Combines knowledge with AI flexibility
  - Can reference documentation
  - Adapts to new patterns
- **Cons:**
  - Requires vector database
  - Complex infrastructure
  - Higher latency
  - Still non-deterministic
- **Rejected because:** Overcomplicated for our use case; static packs provide sufficient flexibility

## Related Decisions

- **ADR-002: Unified Tool Interface** - Knowledge packs enhance AI outputs in generate-dockerfile and generate-k8s-manifests tools
- **ADR-004: Policy-Based Configuration** - Policies provide deterministic constraints while knowledge packs provide AI guidance

## References

- Knowledge packs: `knowledge/packs/`
- Pack loader: `src/knowledge/pack-loader.ts`
- Prompt enhancement: `src/ai/prompt-builder.ts`
- Framework detection: `src/tools/analyze-repo/detectors/`
- Usage in tools: `src/tools/generate-dockerfile/tool.ts`, `src/tools/generate-k8s-manifests/tool.ts`
