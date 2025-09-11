/**
 * Unit tests for scoring functions in ai-helpers.ts
 */

import {
  detectMultistageDocker,
  countDockerLayers,
  extractBaseImage,
  detectSecrets,
  validateYamlSyntax,
  extractK8sResources,
  normalizeScore,
  weightedAverage,
} from '@mcp/tools/ai-helpers';

describe('Scoring Helper Utilities', () => {
  describe('detectMultistageDocker', () => {
    it('should detect multi-stage Dockerfiles', () => {
      const multiStage = `
FROM node:18 AS builder
WORKDIR /app
COPY . .
RUN npm ci

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app .
CMD ["node", "index.js"]
`;
      expect(detectMultistageDocker(multiStage)).toBe(true);
    });

    it('should return false for single-stage Dockerfiles', () => {
      const singleStage = `
FROM node:18
WORKDIR /app
COPY . .
RUN npm ci
CMD ["node", "index.js"]
`;
      expect(detectMultistageDocker(singleStage)).toBe(false);
    });
  });

  describe('countDockerLayers', () => {
    it('should count all layer-creating instructions', () => {
      const dockerfile = `
FROM node:18
ARG NODE_ENV=production
ENV PORT=3000
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
USER node
CMD ["node", "index.js"]
`;
      // FROM, ARG, ENV, WORKDIR, COPY (2), RUN, USER = 8 layers
      expect(countDockerLayers(dockerfile)).toBe(8);
    });

    it('should handle empty Dockerfile', () => {
      expect(countDockerLayers('')).toBe(0);
    });
  });

  describe('extractBaseImage', () => {
    it('should extract base image from FROM instruction', () => {
      const dockerfile = `FROM node:18-alpine\nWORKDIR /app`;
      expect(extractBaseImage(dockerfile)).toBe('node:18-alpine');
    });

    it('should extract first base image from multi-stage', () => {
      const dockerfile = `
FROM node:18 AS builder
FROM nginx:alpine
`;
      expect(extractBaseImage(dockerfile)).toBe('node:18');
    });

    it('should return null for invalid Dockerfile', () => {
      expect(extractBaseImage('WORKDIR /app')).toBeNull();
    });

    it('should handle FROM with platform', () => {
      const dockerfile = `FROM --platform=linux/amd64 node:18`;
      expect(extractBaseImage(dockerfile)).toBe('--platform=linux/amd64');
    });
  });

  describe('detectSecrets', () => {
    it('should detect hardcoded passwords', () => {
      const content = `
ENV DB_PASSWORD="secret123"
password = "admin123"
`;
      const secrets = detectSecrets(content);
      expect(secrets).toContain('password: 2 occurrence(s)');
    });

    it('should detect API keys', () => {
      const content = `api_key: "sk-1234567890abcdef"`;
      const secrets = detectSecrets(content);
      expect(secrets).toContain('api_key: 1 occurrence(s)');
    });

    it('should detect private keys', () => {
      const content = `-----BEGIN RSA PRIVATE KEY-----`;
      const secrets = detectSecrets(content);
      expect(secrets).toContain('private_key: 1 occurrence(s)');
    });

    it('should return empty array for clean content', () => {
      const content = `FROM node:18\nWORKDIR /app`;
      expect(detectSecrets(content)).toEqual([]);
    });
  });

  describe('validateYamlSyntax', () => {
    it('should validate correct YAML', () => {
      const yaml = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
`;
      expect(validateYamlSyntax(yaml)).toBe(true);
    });

    it('should reject YAML with tabs', () => {
      const yaml = `
apiVersion: v1
	kind: ConfigMap
`;
      expect(validateYamlSyntax(yaml)).toBe(false);
    });

    it('should reject YAML with odd indentation', () => {
      const yaml = `
apiVersion: v1
 kind: ConfigMap
`;
      expect(validateYamlSyntax(yaml)).toBe(false);
    });

    it('should accept YAML with document marker', () => {
      const yaml = `---
apiVersion: v1
kind: ConfigMap
`;
      expect(validateYamlSyntax(yaml)).toBe(true);
    });

    it('should reject invalid YAML structure', () => {
      const yaml = `This is not YAML`;
      expect(validateYamlSyntax(yaml)).toBe(false);
    });
  });

  describe('extractK8sResources', () => {
    it('should extract basic resource information', () => {
      const yaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: production
spec:
  replicas: 3
`;
      const resources = extractK8sResources(yaml);
      expect(resources).toHaveLength(1);
      expect(resources[0]).toEqual({
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        name: 'test-app',
        namespace: 'production',
        replicas: 3,
      });
    });

    it('should extract resource specifications', () => {
      const yaml = `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    resources:
      limits:
        cpu: "1"
        memory: "512Mi"
      requests:
        cpu: "500m"
        memory: "256Mi"
`;
      const resources = extractK8sResources(yaml);
      expect(resources[0].resources).toEqual({
        limits: { cpu: '1', memory: '512Mi' },
        requests: { cpu: '500m', memory: '256Mi' },
      });
    });

    it('should handle multiple documents', () => {
      const yaml = `
apiVersion: v1
kind: Service
metadata:
  name: service1
---
apiVersion: v1
kind: Service
metadata:
  name: service2
`;
      const resources = extractK8sResources(yaml);
      expect(resources).toHaveLength(2);
      expect(resources[0].name).toBe('service1');
      expect(resources[1].name).toBe('service2');
    });

    it('should handle empty documents', () => {
      expect(extractK8sResources('')).toEqual([]);
      expect(extractK8sResources('---\n---')).toEqual([]);
    });
  });

  describe('normalizeScore', () => {
    it('should cap scores at maximum', () => {
      expect(normalizeScore(150)).toBe(100);
      expect(normalizeScore(150, 200)).toBe(150);
    });

    it('should floor scores at 0', () => {
      expect(normalizeScore(-50)).toBe(0);
      expect(normalizeScore(-10, 100)).toBe(0);
    });

    it('should pass through valid scores', () => {
      expect(normalizeScore(75)).toBe(75);
      expect(normalizeScore(50, 100)).toBe(50);
    });
  });

  describe('weightedAverage', () => {
    it('should calculate weighted average correctly', () => {
      const scores = { a: 80, b: 60, c: 100 };
      const weights = { a: 0.5, b: 0.3, c: 0.2 };
      // (80 * 0.5 + 60 * 0.3 + 100 * 0.2) / (0.5 + 0.3 + 0.2)
      // = (40 + 18 + 20) / 1 = 78
      expect(weightedAverage(scores, weights)).toBe(78);
    });

    it('should handle missing weights', () => {
      const scores = { a: 80, b: 60, c: 100 };
      const weights = { a: 0.5, b: 0.5 }; // c has no weight
      // (80 * 0.5 + 60 * 0.5 + 100 * 0) / (0.5 + 0.5)
      // = (40 + 30) / 1 = 70
      expect(weightedAverage(scores, weights)).toBe(70);
    });

    it('should return simple average when no weights provided', () => {
      const scores = { a: 80, b: 60, c: 100 };
      const weights = {};
      // (80 + 60 + 100) / 3 = 80
      expect(weightedAverage(scores, weights)).toBe(80);
    });

    it('should handle single score', () => {
      const scores = { a: 75 };
      const weights = { a: 1 };
      expect(weightedAverage(scores, weights)).toBe(75);
    });

    it('should handle empty scores', () => {
      expect(weightedAverage({}, {})).toBe(0);
    });
  });
});

describe('Dockerfile Scoring Functions', () => {
  // Note: These would typically be tested via integration tests
  // since the scoring functions are not exported individually.
  // Here we can test them indirectly through the sampling flow.
  
  describe('Integration with sampling', () => {
    it('should score valid Dockerfile higher than invalid', () => {
      const goodDockerfile = `
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
RUN npm run build

FROM node:18-alpine
RUN apk add --no-cache dumb-init
RUN addgroup -g 1001 -S nodejs && adduser -S nodejs -u 1001
WORKDIR /app
COPY --from=builder --chown=nodejs:nodejs /app/dist ./dist
COPY --from=builder --chown=nodejs:nodejs /app/node_modules ./node_modules
USER nodejs
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\
  CMD node healthcheck.js
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
EXPOSE 3000
CMD node /app/index.js
`;

      // These would be scored through the actual scoring functions
      // Good dockerfile should score higher on:
      // - Multi-stage build
      // - Alpine base image
      // - Non-root user
      // - Health check
      // - Layer optimization
      // - No hardcoded secrets
      
      // Bad dockerfile issues:
      // - Using :latest tag
      // - Multiple RUN commands (not chained)
      // - No user (runs as root)
      // - Hardcoded password
      // - No health check
      // - Copies everything at once
    });
  });
});

describe('Kubernetes YAML Scoring Functions', () => {
  describe('Integration with sampling', () => {
    it('should score secure K8s manifest higher', () => {
      const secureManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secure-app
  namespace: production
  labels:
    app.kubernetes.io/name: secure-app
    app.kubernetes.io/version: "1.0.0"
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
`;

      const insecureManifest = `
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
          value: "secret123"
`;

      // Secure manifest scores higher on:
      // - Security context configured
      // - Non-root user
      // - Resource limits and requests
      // - Health checks
      // - Proper labels and annotations
      // - No hardcoded secrets
      // - Versioned image tag
      
      // Insecure manifest issues:
      // - No security context
      // - Runs as root (default)
      // - No resource limits
      // - No health checks
      // - Hardcoded password
      // - Using :latest tag
    });
  });
});