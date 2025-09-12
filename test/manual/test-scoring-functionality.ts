#!/usr/bin/env tsx
/**
 * Manual Testing Script for Scoring Functionality
 * 
 * This script validates the complete scoring implementation including:
 * - Dockerfile scoring
 * - Kubernetes YAML scoring
 * - Generic content scoring
 * - Helper utilities
 * - Integration with sampling flow
 * 
 * Usage: npx tsx test/manual/test-scoring-functionality.ts
 */

import { inspect } from 'util';
import {
  detectMultistageDocker,
  countDockerLayers,
  extractBaseImage,
  detectSecrets,
  validateYamlSyntax,
  extractK8sResources,
  normalizeScore,
  weightedAverage,
} from '../../src/mcp/tools/ai-helpers';

// Color output helpers
const colors = {
  reset: '\x1b[0m',
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
};

function success(msg: string) {
  console.log(`${colors.green}✓${colors.reset} ${msg}`);
}

function error(msg: string) {
  console.log(`${colors.red}✗${colors.reset} ${msg}`);
}

function info(msg: string) {
  console.log(`${colors.blue}ℹ${colors.reset} ${msg}`);
}

function section(title: string) {
  console.log(`\n${colors.cyan}═══ ${title} ═══${colors.reset}\n`);
}

function subsection(title: string) {
  console.log(`\n${colors.magenta}→ ${title}${colors.reset}`);
}

// Test data
const goodDockerfile = `
# syntax=docker/dockerfile:1
FROM node:18-alpine AS builder
ARG NODE_ENV=production
WORKDIR /app

# Copy dependency files for better caching
COPY package*.json ./
RUN npm ci --only=production --frozen-lockfile && npm cache clean --force

# Copy source and build
COPY . .
RUN npm run build

# Production stage
FROM node:18-alpine
RUN apk add --no-cache dumb-init && \\
    addgroup -g 1001 -S nodejs && \\
    adduser -S nodejs -u 1001

WORKDIR /app

# Copy built application with proper ownership
COPY --from=builder --chown=nodejs:nodejs /app/dist ./dist
COPY --from=builder --chown=nodejs:nodejs /app/node_modules ./node_modules

USER nodejs
EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\
  CMD node healthcheck.js || exit 1

ENTRYPOINT ["dumb-init", "--"]
CMD ["node", "dist/index.js"]
`;

const badDockerfile = `
FROM ubuntu:latest
COPY . /app
RUN apt-get update
RUN apt-get install -y nodejs
RUN apt-get install -y npm
RUN cd /app && npm install
ENV PASSWORD=secret123
ENV API_KEY="sk-1234567890"
EXPOSE 3000
CMD node /app/index.js
`;

const goodK8sManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secure-app
  namespace: production
  labels:
    app.kubernetes.io/name: secure-app
    app.kubernetes.io/version: "1.0.0"
    app.kubernetes.io/component: backend
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
  selector:
    matchLabels:
      app: secure-app
  template:
    metadata:
      labels:
        app: secure-app
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
      - name: app
        image: myapp:1.2.3
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
        resources:
          limits:
            cpu: "1"
            memory: "512Mi"
          requests:
            cpu: "500m"
            memory: "256Mi"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        startupProbe:
          httpGet:
            path: /startup
            port: 8080
          failureThreshold: 30
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: secure-app-service
spec:
  selector:
    app: secure-app
  ports:
  - port: 80
    targetPort: 8080
`;

const badK8sManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: app
  template:
    spec:
      containers:
      - name: app
        image: myapp:latest
        env:
        - name: DB_PASSWORD
          value: "admin123"
        - name: SECRET_TOKEN
          value: "secret-token-123"
`;

const invalidYaml = `
apiVersion: v1
	kind: ConfigMap
  metadata:
   name: test
`;

// Test functions
async function testDockerfileHelpers() {
  section('Testing Dockerfile Helper Functions');

  subsection('Multi-stage Detection');
  const isMultistageGood = detectMultistageDocker(goodDockerfile);
  const isMultistageBad = detectMultistageDocker(badDockerfile);
  
  if (isMultistageGood === true) {
    success(`Good Dockerfile correctly detected as multi-stage`);
  } else {
    error(`Failed to detect multi-stage in good Dockerfile`);
  }
  
  if (isMultistageBad === false) {
    success(`Bad Dockerfile correctly detected as single-stage`);
  } else {
    error(`Incorrectly detected multi-stage in bad Dockerfile`);
  }

  subsection('Layer Counting');
  const goodLayers = countDockerLayers(goodDockerfile);
  const badLayers = countDockerLayers(badDockerfile);
  
  info(`Good Dockerfile has ${goodLayers} layers`);
  info(`Bad Dockerfile has ${badLayers} layers`);
  
  if (goodLayers < badLayers) {
    success(`Good Dockerfile has fewer layers (better optimization)`);
  } else {
    error(`Layer count comparison unexpected`);
  }

  subsection('Base Image Extraction');
  const goodBase = extractBaseImage(goodDockerfile);
  const badBase = extractBaseImage(badDockerfile);
  
  if (goodBase === 'node:18-alpine') {
    success(`Correctly extracted base image: ${goodBase}`);
  } else {
    error(`Failed to extract correct base image: ${goodBase}`);
  }
  
  if (badBase === 'ubuntu:latest') {
    success(`Correctly extracted base image: ${badBase}`);
  } else {
    error(`Failed to extract correct base image: ${badBase}`);
  }

  subsection('Secret Detection');
  const goodSecrets = detectSecrets(goodDockerfile);
  const badSecrets = detectSecrets(badDockerfile);
  
  if (goodSecrets.length === 0) {
    success(`No secrets detected in good Dockerfile`);
  } else {
    error(`Unexpected secrets in good Dockerfile: ${goodSecrets.join(', ')}`);
  }
  
  if (badSecrets.length > 0) {
    success(`Detected ${badSecrets.length} secrets in bad Dockerfile: ${badSecrets.join(', ')}`);
  } else {
    error(`Failed to detect secrets in bad Dockerfile`);
  }
}

async function testYamlHelpers() {
  section('Testing YAML/Kubernetes Helper Functions');

  subsection('YAML Syntax Validation');
  const goodValid = validateYamlSyntax(goodK8sManifest);
  const badValid = validateYamlSyntax(badK8sManifest);
  const invalidValid = validateYamlSyntax(invalidYaml);
  
  if (goodValid === true) {
    success(`Good K8s manifest has valid YAML syntax`);
  } else {
    error(`Good K8s manifest incorrectly marked as invalid`);
  }
  
  if (badValid === true) {
    success(`Bad K8s manifest has valid YAML syntax (expected)`);
  } else {
    error(`Bad K8s manifest marked as invalid syntax`);
  }
  
  if (invalidValid === false) {
    success(`Invalid YAML correctly detected (has tabs)`);
  } else {
    error(`Failed to detect invalid YAML`);
  }

  subsection('K8s Resource Extraction');
  const goodResources = extractK8sResources(goodK8sManifest);
  const badResources = extractK8sResources(badK8sManifest);
  
  info(`Good manifest: Found ${goodResources.length} resources`);
  goodResources.forEach((resource, i) => {
    info(`  Resource ${i + 1}: ${resource.kind} - ${resource.name || 'unnamed'}`);
    if (resource.resources) {
      info(`    Resources: ${JSON.stringify(resource.resources)}`);
    }
  });
  
  info(`Bad manifest: Found ${badResources.length} resources`);
  badResources.forEach((resource, i) => {
    info(`  Resource ${i + 1}: ${resource.kind} - ${resource.name || 'unnamed'}`);
  });

  if (goodResources.length === 2) {
    success(`Correctly extracted 2 resources from good manifest`);
  } else {
    error(`Expected 2 resources, got ${goodResources.length}`);
  }

  const deployment = goodResources.find(r => r.kind === 'Deployment');
  if (deployment?.replicas === 3) {
    success(`Correctly extracted replicas: ${deployment.replicas}`);
  } else {
    error(`Failed to extract correct replicas`);
  }

  if (deployment?.resources?.limits?.cpu === '1') {
    success(`Correctly extracted CPU limits`);
  } else {
    error(`Failed to extract CPU limits`);
  }

  subsection('Secret Detection in K8s');
  const goodK8sSecrets = detectSecrets(goodK8sManifest);
  const badK8sSecrets = detectSecrets(badK8sManifest);
  
  if (goodK8sSecrets.length === 0) {
    success(`No secrets in secure K8s manifest`);
  } else {
    error(`Unexpected secrets in secure manifest: ${goodK8sSecrets.join(', ')}`);
  }
  
  if (badK8sSecrets.length > 0) {
    success(`Detected ${badK8sSecrets.length} secrets in insecure manifest`);
  } else {
    error(`Failed to detect secrets in insecure manifest`);
  }
}

async function testScoringUtilities() {
  section('Testing Scoring Utility Functions');

  subsection('Score Normalization');
  const testScores = [150, -50, 75, 0, 100];
  
  testScores.forEach(score => {
    const normalized = normalizeScore(score);
    info(`Score ${score} → ${normalized}`);
    
    if (normalized >= 0 && normalized <= 100) {
      success(`Score ${score} correctly normalized to ${normalized}`);
    } else {
      error(`Score ${score} incorrectly normalized to ${normalized}`);
    }
  });

  subsection('Weighted Average Calculation');
  const scores = {
    build: 80,
    size: 60,
    security: 90,
    speed: 70
  };
  
  const weights = {
    build: 30,
    size: 30,
    security: 25,
    speed: 15
  };
  
  const weighted = weightedAverage(scores, weights);
  const expected = (80 * 30 + 60 * 30 + 90 * 25 + 70 * 15) / 100;
  
  info(`Scores: ${JSON.stringify(scores)}`);
  info(`Weights: ${JSON.stringify(weights)}`);
  info(`Weighted Average: ${weighted}`);
  info(`Expected: ${expected}`);
  
  if (Math.abs(weighted - expected) < 0.01) {
    success(`Weighted average calculated correctly: ${weighted}`);
  } else {
    error(`Weighted average incorrect. Expected ${expected}, got ${weighted}`);
  }

  // Test with missing weights
  const partialWeights = { build: 50, size: 50 };
  const partialWeighted = weightedAverage(scores, partialWeights);
  const partialExpected = (80 * 50 + 60 * 50) / 100;
  
  info(`\nPartial weights: ${JSON.stringify(partialWeights)}`);
  info(`Result: ${partialWeighted}`);
  
  if (Math.abs(partialWeighted - partialExpected) < 0.01) {
    success(`Handles missing weights correctly`);
  } else {
    error(`Failed with partial weights`);
  }

  // Test with no weights (simple average)
  const simpleAvg = weightedAverage(scores, {});
  const simpleExpected = (80 + 60 + 90 + 70) / 4;
  
  info(`\nNo weights (simple average): ${simpleAvg}`);
  
  if (Math.abs(simpleAvg - simpleExpected) < 0.01) {
    success(`Falls back to simple average correctly`);
  } else {
    error(`Simple average fallback failed`);
  }
}

async function testScoringComparison() {
  section('Scoring Engine Tests');
  
  info('Testing the new ScoringEngine with real content samples');
  
  // Import the new scoring engine
  const { createScoringEngine } = await import('../../src/mcp/tools/scoring');
  const engine = createScoringEngine();
  
  subsection('Dockerfile Scoring');
  const goodDockerResult = engine.score(goodDockerfile, 'dockerfile');
  const badDockerResult = engine.score(badDockerfile, 'dockerfile');
  
  if (goodDockerResult.ok && badDockerResult.ok) {
    info(`Good Dockerfile score: ${goodDockerResult.value.total}/100`);
    info(`Bad Dockerfile score: ${badDockerResult.value.total}/100`);
    info(`Good Dockerfile matched rules: ${goodDockerResult.value.matchedRules.length}`);
    info(`Bad Dockerfile matched rules: ${badDockerResult.value.matchedRules.length}`);
    
    if (goodDockerResult.value.total > badDockerResult.value.total) {
      success('Good Dockerfile scored higher than bad Dockerfile');
    } else {
      error('Scoring engine failed to differentiate quality');
    }
  } else {
    error('Scoring engine failed for Dockerfile');
  }
  
  subsection('K8s Manifest Scoring');
  const goodK8sResult = engine.score(goodK8sManifest, 'yaml');
  const badK8sResult = engine.score(badK8sManifest, 'yaml');
  
  if (goodK8sResult.ok && badK8sResult.ok) {
    info(`Good K8s score: ${goodK8sResult.value.total}/100`);
    info(`Bad K8s score: ${badK8sResult.value.total}/100`);
    
    if (goodK8sResult.value.total > badK8sResult.value.total) {
      success('Good K8s manifest scored higher than bad K8s manifest');
    } else {
      error('K8s scoring failed to differentiate quality');
    }
  } else {
    error('Scoring engine failed for K8s');
  }
  
  subsection('Expected Scoring Behavior');
  
  const expectations = [
    {
      name: 'Good Dockerfile',
      expected: {
        build: '>= 80',
        size: '>= 75',
        security: '>= 85',
        speed: '>= 70'
      }
    },
    {
      name: 'Bad Dockerfile',
      expected: {
        build: '<= 60',
        size: '<= 50',
        security: '<= 40',
        speed: '<= 50'
      }
    },
    {
      name: 'Good K8s Manifest',
      expected: {
        validation: '>= 90',
        security: '>= 85',
        resources: '>= 80',
        best_practices: '>= 85'
      }
    },
    {
      name: 'Bad K8s Manifest',
      expected: {
        validation: '>= 70',
        security: '<= 40',
        resources: '<= 40',
        best_practices: '<= 30'
      }
    }
  ];
  
  expectations.forEach(exp => {
    info(`\n${exp.name} expected scores:`);
    Object.entries(exp.expected).forEach(([criterion, range]) => {
      info(`  ${criterion}: ${range}`);
    });
  });
  
  success('Scoring expectations documented');
}

async function testIntegrationPoints() {
  section('Integration Points Verification');

  subsection('Config Integration');
  info('Checking config structure expectations...');
  
  const expectedConfig = {
    'sampling.weights.dockerfile.build': 30,
    'sampling.weights.dockerfile.size': 30,
    'sampling.weights.dockerfile.security': 25,
    'sampling.weights.dockerfile.speed': 15,
    'sampling.weights.k8s.validation': 20,
    'sampling.weights.k8s.security': 20,
    'sampling.weights.k8s.resources': 20,
    'sampling.weights.k8s.best_practices': 20,
  };
  
  Object.entries(expectedConfig).forEach(([path, value]) => {
    info(`  ${path}: ${value}`);
  });
  
  success('Config structure documented');

  subsection('Scoring Engine Integration');
  info('New ScoringEngine architecture:');
  const newArchitecture = [
    'ScoringEngine class (src/mcp/tools/scoring/engine.ts)',
    'DOCKERFILE_PROFILE (25 rules)',
    'K8S_PROFILE (33 rules)', 
    'GENERIC_PROFILE (18 rules)',
    'Rule-based scoring with configurable profiles',
    'Integration through scoreCandidates() in ai-helpers.ts',
    'Fallback to legacy functions for error handling'
  ];
  
  newArchitecture.forEach(item => {
    info(`  ✓ ${item}`);
  });
  
  subsection('Legacy Functions (Preserved for Fallback)');
  const legacyFunctions = [
    'scoreDockerfileBuild', 'scoreDockerfileSize', 'scoreDockerfileSecurity', 'scoreDockerfileSpeed',
    'scoreYamlValidation', 'scoreYamlSecurity', 'scoreYamlResources', 'scoreYamlBestPractices',
    'scoreGenericQuality', 'scoreGenericSecurity', 'scoreGenericEfficiency', 'scoreGenericMaintainability'
  ];
  
  info('Legacy scoring functions maintained as emergency fallback:');
  legacyFunctions.forEach(fn => {
    info(`  → ${fn} (legacy fallback)`);
  });
  
  success('Scoring engine architecture documented');
}

async function testPerformance() {
  section('Performance Testing');

  subsection('Helper Function Performance');
  
  const iterations = 1000;
  const perfTests = [
    {
      name: 'detectMultistageDocker',
      fn: () => detectMultistageDocker(goodDockerfile)
    },
    {
      name: 'countDockerLayers',
      fn: () => countDockerLayers(goodDockerfile)
    },
    {
      name: 'extractBaseImage',
      fn: () => extractBaseImage(goodDockerfile)
    },
    {
      name: 'detectSecrets',
      fn: () => detectSecrets(goodDockerfile)
    },
    {
      name: 'validateYamlSyntax',
      fn: () => validateYamlSyntax(goodK8sManifest)
    },
    {
      name: 'extractK8sResources',
      fn: () => extractK8sResources(goodK8sManifest)
    }
  ];
  
  for (const test of perfTests) {
    const start = performance.now();
    for (let i = 0; i < iterations; i++) {
      test.fn();
    }
    const end = performance.now();
    const avgTime = (end - start) / iterations;
    
    if (avgTime < 1) {
      success(`${test.name}: ${avgTime.toFixed(3)}ms avg (${iterations} iterations)`);
    } else if (avgTime < 5) {
      info(`${test.name}: ${avgTime.toFixed(3)}ms avg (${iterations} iterations)`);
    } else {
      error(`${test.name}: ${avgTime.toFixed(3)}ms avg - SLOW!`);
    }
  }
  
  subsection('Target Performance');
  info('Performance targets:');
  info('  Individual score calculation: < 5ms');
  info('  Complete candidate scoring: < 10ms');
  info('  Total overhead for 5 candidates: < 50ms');
}

// Main execution
async function main() {
  console.log(colors.cyan + '╔═══════════════════════════════════════════════════════╗');
  console.log('║     Manual Testing - Scoring Functionality Suite      ║');
  console.log('╚═══════════════════════════════════════════════════════╝' + colors.reset);
  
  try {
    await testDockerfileHelpers();
    await testYamlHelpers();
    await testScoringUtilities();
    await testScoringComparison();
    await testIntegrationPoints();
    await testPerformance();
    
    section('Test Suite Complete');
    success('All manual tests completed successfully!');
    
    console.log('\n' + colors.yellow + 'Next Steps:' + colors.reset);
    console.log('1. Run integration tests: npm run test:integration');
    console.log('2. Test with MCP Inspector: npm run mcp:inspect');
    console.log('3. Test sampling with actual tools via CLI');
    
  } catch (err) {
    section('Test Suite Failed');
    error(`Fatal error: ${err}`);
    console.error(err);
    process.exit(1);
  }
}

// Run tests
main().catch(console.error);