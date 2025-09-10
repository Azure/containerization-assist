import type { AITemplate } from './types';

export const OPTIMIZATION_SUGGESTION: AITemplate = {
  id: 'optimization-suggestion',
  name: 'Container Optimization Suggestions',
  description: 'Provide optimization recommendations for Docker images and deployments',
  version: '1.0.0',
  system:
    'You are an expert in container optimization, focusing on:\n- Image size reduction\n- Build time optimization\n- Security hardening\n- Performance tuning\n- Cost optimization\nProvide specific, actionable recommendations with measurable impact.\n',
  user: 'Analyze and provide optimization suggestions for this containerized application:\n\n{{#if dockerfile}}\nCurrent Dockerfile:\n```dockerfile\n{{dockerfile}}\n```\n{{/if}}\n\n{{#if imageInfo}}\nImage Information:\n- Size: {{imageSize}}\n- Layers: {{layerCount}}\n- Base Image: {{baseImage}}\n{{/if}}\n\n{{#if scanResults}}\nSecurity Scan Results:\n- Critical: {{criticalVulns}}\n- High: {{highVulns}}\n- Medium: {{mediumVulns}}\n{{/if}}\n\nApplication Details:\n- Language: {{language}}\n- Framework: {{framework}}\n- Build System: {{buildSystem}}\n{{#if requirements}}\n- Requirements: {{requirements}}\n{{/if}}\n\nProvide optimization recommendations for:\n1. Image size reduction\n2. Build performance\n3. Security improvements\n4. Runtime performance\n5. Cost optimization\n',
  variables: [
    {
      name: 'dockerfile',
      description: 'Current Dockerfile to optimize',
      required: false,
    },
    {
      name: 'imageSize',
      description: 'Current image size',
      required: false,
      default: 'unknown',
    },
    {
      name: 'layerCount',
      description: 'Number of layers in image',
      required: false,
    },
    {
      name: 'baseImage',
      description: 'Current base image',
      required: false,
    },
    {
      name: 'criticalVulns',
      description: 'Number of critical vulnerabilities',
      required: false,
      default: '0',
    },
    {
      name: 'highVulns',
      description: 'Number of high vulnerabilities',
      required: false,
      default: '0',
    },
    {
      name: 'mediumVulns',
      description: 'Number of medium vulnerabilities',
      required: false,
      default: '0',
    },
    {
      name: 'language',
      description: 'Programming language',
      required: true,
    },
    {
      name: 'framework',
      description: 'Application framework',
      required: false,
    },
    {
      name: 'buildSystem',
      description: 'Build system',
      required: true,
    },
    {
      name: 'requirements',
      description: 'Special requirements or constraints',
      required: false,
    },
  ],
  outputFormat: 'text',
  examples: [
    {
      input: {
        language: 'java',
        framework: 'spring-boot',
        buildSystem: 'maven',
        imageSize: '850MB',
        layerCount: 15,
        baseImage: 'openjdk:11',
      },
      output:
        '## Container Optimization Recommendations\n\n### 1. Image Size Reduction (Potential: 60-70% reduction)\n\n**Current Issue**: Using full JDK image (850MB) when JRE would suffice\n\n**Recommendations**:\n- Switch to `eclipse-temurin:11-jre-alpine` (reduces ~500MB)\n- Use multi-stage build to exclude build tools from final image\n- Remove Maven cache and unnecessary files after build\n- Consider using `jlink` to create custom JRE (Java 11+)\n\n```dockerfile\n# Optimized multi-stage build\nFROM maven:3.8-eclipse-temurin-11 AS builder\nWORKDIR /app\nCOPY pom.xml .\nRUN mvn dependency:go-offline\nCOPY src ./src\nRUN mvn package -DskipTests && \\\n    rm -rf /root/.m2\n\nFROM eclipse-temurin:11-jre-alpine\nRUN apk add --no-cache dumb-init\nWORKDIR /app\nCOPY --from=builder /app/target/*.jar app.jar\nENTRYPOINT ["dumb-init", "java", "-jar", "app.jar"]\n```\n\n### 2. Build Performance (Potential: 40-50% faster)\n\n**Improvements**:\n- Cache Maven dependencies in separate layer\n- Use BuildKit cache mounts for Maven repository\n- Parallel build with `-T 1C` flag\n- Use `.dockerignore` to exclude unnecessary files\n\n### 3. Security Improvements\n\n**Critical Actions**:\n- Add non-root user: `RUN adduser -D -u 1000 appuser`\n- Update base image to latest patch version\n- Scan with Trivy/Snyk in CI pipeline\n- Use distroless images for production\n\n### 4. Runtime Performance\n\n**JVM Optimizations**:\n- Set container-aware JVM flags: `-XX:MaxRAMPercentage=75.0`\n- Use G1GC for containers: `-XX:+UseG1GC`\n- Enable Class Data Sharing: `-XX:+UseAppCDS`\n- Implement proper health checks and readiness probes\n\n### 5. Cost Optimization\n\n**Resource Efficiency**:\n- Right-size memory limits based on profiling\n- Use spot instances for non-critical workloads\n- Implement horizontal pod autoscaling\n- Consider ARM-based instances (Graviton) for 20-40% cost savings\n\n**Estimated Impact**:\n- Image size: 850MB → 250MB (70% reduction)\n- Build time: 5min → 2.5min (50% reduction)\n- Memory usage: 512MB → 384MB (25% reduction)\n- Monthly cost: $100 → $65 (35% reduction)\n',
    },
  ],
  tags: ['optimization', 'performance', 'security', 'cost-reduction'],
} as const;
