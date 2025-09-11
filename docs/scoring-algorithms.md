# Scoring Algorithms Documentation

## Overview

The containerization-assist MCP server implements sophisticated scoring algorithms to evaluate and rank multiple candidate outputs when sampling is enabled. This document details the scoring methodology, criteria, and weight configurations.

## Architecture

### Core Components

1. **Scoring Functions** - Content-type specific evaluation functions
2. **Weight Configuration** - Customizable importance factors for each criterion
3. **Score Normalization** - Ensures all scores are in 0-100 range
4. **Weighted Averaging** - Combines individual scores into final rankings

## Dockerfile Scoring

### Scoring Dimensions

#### 1. Build Score (30% default weight)
Evaluates build efficiency and best practices:
- **Multi-stage builds** (+15 points): Reduces final image size
- **Layer optimization** (+10 points): Uses `&&` chaining to minimize layers
- **Dependency caching** (+10 points): Copies package files before source
- **Build arguments** (+5 points): Enables build-time configuration
- **WORKDIR usage** (+10 points): Proper directory management

#### 2. Size Score (30% default weight)
Assesses image size optimization:
- **Alpine/distroless base** (+20-25 points): Minimal base images
- **Cleanup operations** (+15 points): Removes unnecessary files
- **Layer consolidation** (+10 points): Fewer than 3 RUN commands
- **Multi-stage copying** (+10 points): Uses `COPY --from=`
- **No-install-recommends** (+5 points): Avoids optional packages

#### 3. Security Score (25% default weight)
Evaluates security best practices:
- **Non-root user** (+25 points): Runs as unprivileged user
- **No hardcoded secrets** (+15 points): Avoids passwords/tokens
- **Proper file copying** (+10 points): Doesn't copy everything early
- **Health checks** (+10 points): Container monitoring
- **Versioned base images** (+10 points): No `:latest` tags

#### 4. Speed Score (15% default weight)
Measures build speed optimization:
- **Dependency caching** (+20 points): Leverages Docker layer cache
- **Parallel operations** (+15 points): Uses parallel builds
- **Minimal base images** (+15 points): Faster pulls
- **BuildKit features** (+5 points): Modern Docker features
- **Optimal layer count** (+5 points): Balance between caching and size

### Scoring Example

```dockerfile
# High-scoring Dockerfile (85-95 points)
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production --frozen-lockfile
COPY . .
RUN npm run build

FROM node:18-alpine
RUN addgroup -g 1001 -S nodejs && adduser -S nodejs -u 1001
WORKDIR /app
COPY --from=builder --chown=nodejs:nodejs /app/dist ./dist
USER nodejs
HEALTHCHECK CMD node healthcheck.js
CMD ["node", "dist/index.js"]
```

## Kubernetes YAML Scoring

### Scoring Dimensions

#### 1. Validation Score (20% default weight)
Checks YAML structure and K8s compliance:
- **API version present** (+15 points)
- **Kind specified** (+15 points)
- **Metadata complete** (+15 points)
- **Spec defined** (+10 points)
- **Consistent indentation** (+7 points)

#### 2. Security Score (20% default weight)
Evaluates security configurations:
- **Security context** (+15 points)
- **Non-root containers** (+10 points)
- **Read-only filesystem** (+10 points)
- **No privilege escalation** (+8 points)
- **Capabilities dropped** (+7 points)

#### 3. Resources Score (20% default weight)
Assesses resource management:
- **CPU/memory limits** (+20 points)
- **CPU/memory requests** (+20 points)
- **Autoscaling configured** (+5 points)
- **Storage specifications** (+10 points)

#### 4. Best Practices Score (20% default weight)
Checks Kubernetes best practices:
- **Liveness probe** (+10 points)
- **Readiness probe** (+10 points)
- **Labels and annotations** (+13 points)
- **Update strategy** (+8 points)
- **Pod disruption budget** (+5 points)

## Generic Content Scoring

For non-specific content types:

### Quality Score (40% weight)
- Structure and completeness
- Documentation/comments
- Formatting consistency
- Pattern usage

### Security Score (30% weight)
- No hardcoded credentials
- Environment variable usage
- Permission restrictions

### Efficiency Score (20% weight)
- Content density
- No excessive repetition
- Efficient patterns

### Maintainability Score (10% weight)
- Readable structure
- Consistent indentation
- Meaningful naming
- Version indicators

## Weight Configuration

Weights are configurable via `src/config/index.ts`:

```typescript
sampling: {
  weights: {
    dockerfile: {
      build: 30,      // Build efficiency
      size: 30,       // Image size optimization
      security: 25,   // Security best practices
      speed: 15       // Build speed
    },
    k8s: {
      validation: 20,     // YAML validation
      security: 20,       // Security configuration
      resources: 20,      // Resource management
      best_practices: 20  // K8s best practices
    }
  }
}
```

## Scoring Algorithm

### 1. Individual Scoring
Each criterion is evaluated independently:
```typescript
score = baseScore + sum(feature_points)
normalizedScore = min(max(score, 0), 100)
```

### 2. Weighted Combination
Final score combines all criteria:
```typescript
finalScore = Σ(criterion_score × criterion_weight) / Σ(weights)
```

### 3. Candidate Ranking
Candidates are sorted by final score:
```typescript
candidates.sort((a, b) => b.score - a.score)
candidates.forEach((c, i) => c.rank = i + 1)
```

## Early Stopping

The system can stop generating candidates early if a high-quality candidate is found:

1. **Quick Score**: Fast heuristic evaluation
2. **Threshold Check**: Compare against `earlyStopThreshold` (default 90)
3. **Stop Decision**: Skip remaining candidates if threshold met

## Performance Considerations

### Scoring Performance Targets
- Individual score calculation: < 5ms
- Complete candidate scoring: < 10ms
- Total overhead for 5 candidates: < 50ms

### Optimization Techniques
1. **Regex Compilation**: Pre-compile patterns where possible
2. **Early Returns**: Skip expensive checks when score is already determined
3. **Caching**: Cache repeated pattern matches within same content

## Tuning Guidelines

### Adjusting Weights
1. **Increase security weight**: For production environments
2. **Increase speed weight**: For CI/CD pipelines
3. **Increase size weight**: For edge deployments
4. **Balance all weights**: For general-purpose use

### Custom Scoring
To add custom scoring criteria:

1. Add new scoring function in `ai-helpers.ts`
2. Update `calculateScoreBreakdown` to call new function
3. Add weight configuration in `config/index.ts`
4. Document scoring logic here

## Validation and Testing

### Unit Tests
Located in `test/unit/mcp/tools/scoring.test.ts`:
- Helper utility tests
- Score normalization tests
- Weight calculation tests

### Integration Tests
- End-to-end sampling with scoring
- Comparison of good vs bad candidates
- Early stopping behavior

## Future Enhancements

1. **Machine Learning Integration**: Train scoring models on user feedback
2. **Custom Scoring Plugins**: Allow users to define custom scoring functions
3. **A/B Testing Framework**: Compare scoring algorithm effectiveness
4. **Performance Analytics**: Track scoring accuracy over time
5. **Context-Aware Scoring**: Adjust weights based on project type

## API Reference

### Exported Functions

```typescript
// Detect multi-stage Docker builds
detectMultistageDocker(content: string): boolean

// Count Docker layers
countDockerLayers(content: string): number

// Extract base image
extractBaseImage(content: string): string | null

// Detect secrets
detectSecrets(content: string): string[]

// Validate YAML syntax
validateYamlSyntax(content: string): boolean

// Extract K8s resources
extractK8sResources(content: string): ResourceSpec[]

// Normalize scores
normalizeScore(score: number, max?: number): number

// Calculate weighted average
weightedAverage(scores: Record<string, number>, weights: Record<string, number>): number
```

## Conclusion

The scoring system provides intelligent candidate selection through comprehensive evaluation across multiple dimensions. By combining heuristic analysis with configurable weights, it ensures the best possible output while maintaining performance and flexibility.