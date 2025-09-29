import {
  sampleCandidates,
  sampleWithCache,
  sample,
  scoreDockerfile,
  scoreKubernetesManifest,
  scoreHelmChart,
  scoreACAManifest,
  scoreBaseImageRecommendation,
  scoreContent,
  type SamplingCandidate,
} from '@/lib/sampling';

describe('Sampling Functions', () => {
  describe('sampleCandidates', () => {
    it('should generate and score candidates', async () => {
      let callCount = 0;
      const generate = async () => {
        callCount++;
        return `Content ${callCount}`;
      };
      const score = (content: string) => {
        return content.includes('1') ? 80 : 60;
      };

      const result = await sampleCandidates(generate, score, { count: 3 });

      expect(result).toBeDefined();
      expect(result.content).toBe('Content 1');
      expect(result.score).toBe(80);
      expect(result.rank).toBe(1);
      expect(callCount).toBe(3);
    });

    it('should stop early when hitting high score', async () => {
      let callCount = 0;
      const generate = async () => {
        callCount++;
        return `Content ${callCount}`;
      };
      const score = (content: string) => {
        return content.includes('2') ? 96 : 60;
      };

      const result = await sampleCandidates(generate, score, { count: 5, stopAt: 95 });

      expect(result).toBeDefined();
      expect(result.score).toBe(96);
      expect(callCount).toBe(2); // Should stop after second candidate
    });

    it('should handle score breakdown', async () => {
      const generate = async () => 'Test content';
      const score = () => ({
        quality: 80,
        performance: 70,
        security: 90,
      });

      const result = await sampleCandidates(generate, score, { count: 1 });

      expect(result.score).toBe(80); // Average of scores
      expect(result.scoreBreakdown).toEqual({
        quality: 80,
        performance: 70,
        security: 90,
      });
    });
  });

  describe('sampleWithCache', () => {
    it('should cache results', async () => {
      let generateCount = 0;
      const generate = async () => {
        generateCount++;
        return `Generated ${generateCount}`;
      };
      const score = () => 75;

      const result1 = await sampleWithCache('test-key', generate, score);
      const result2 = await sampleWithCache('test-key', generate, score);

      expect(result1.content).toBe('Generated 1');
      expect(result2.content).toBe('Generated 1');
      expect(generateCount).toBe(3); // 3 candidates generated only once
    });

    it('should skip cache when disabled', async () => {
      let generateCount = 0;
      const generate = async () => {
        generateCount++;
        return `Generated ${generateCount}`;
      };
      const score = () => 75;

      await sampleWithCache('test-key-2', generate, score, { useCache: false });
      await sampleWithCache('test-key-2', generate, score, { useCache: false });

      expect(generateCount).toBe(6); // 3 candidates * 2 calls
    });
  });

  describe('sample with metadata', () => {
    it('should return complete sampling result', async () => {
      const generate = async () => 'Test content';
      const score = () => 85;
      const transform = (candidate: SamplingCandidate) => ({
        content: candidate.content,
        processed: true,
      });

      const result = await sample(generate, score, transform, {
        maxCandidates: 2,
        returnAllCandidates: true,
        includeScoreBreakdown: true,
      });

      expect(result.winner).toBeDefined();
      expect(result.winner.score).toBe(85);
      expect(result.winner.processed).toBe(true);
      expect(result.allCandidates).toHaveLength(2);
      expect(result.samplingMetadata).toBeDefined();
      expect(result.samplingMetadata?.candidatesGenerated).toBe(2);
      expect(result.samplingMetadata?.samplingDuration).toBeGreaterThanOrEqual(0);
    });

    it('should handle early stopping with metadata', async () => {
      let count = 0;
      const generate = async () => {
        count++;
        return `Content ${count}`;
      };
      const score = (content: string) => (content.includes('2') ? 98 : 60);
      const transform = (c: SamplingCandidate) => ({ content: c.content });

      const result = await sample(generate, score, transform, {
        maxCandidates: 5,
        earlyStopThreshold: 95,
      });

      expect(result.winner.score).toBe(98);
      expect(result.samplingMetadata?.stoppedEarly).toBe(true);
      expect(result.samplingMetadata?.candidatesGenerated).toBe(2);
    });
  });

  describe('Scoring Functions', () => {
    describe('scoreDockerfile', () => {
      it('should score Dockerfile content', () => {
        const dockerfile = `
FROM node:16-alpine
RUN apk add --no-cache git
USER node
WORKDIR /app
COPY --chown=node:node package*.json ./
RUN npm ci
COPY --chown=node:node . .
HEALTHCHECK CMD node healthcheck.js
CMD ["node", "server.js"]
`;
        const scores = scoreDockerfile(dockerfile);

        expect(scores.size).toBeGreaterThan(0);
        expect(scores.security).toBeGreaterThan(0);
        expect(scores.bestPractices).toBeGreaterThan(0);
        expect(scores.caching).toBeGreaterThanOrEqual(0);
      });

      it('should give high scores for optimized Dockerfiles', () => {
        const optimized = `
FROM node:alpine AS builder
COPY package*.json ./
RUN npm ci --no-cache && rm -rf /tmp/*

FROM node:alpine
USER node
COPY --from=builder --chown=node:node /app/node_modules ./node_modules
COPY --chown=node:node . .
HEALTHCHECK CMD curl -f http://localhost:3000/health
LABEL version="1.0"
CMD ["node", "server.js"]
`;
        const scores = scoreDockerfile(optimized);

        expect(scores.size).toBeGreaterThan(50);
        expect(scores.security).toBeGreaterThan(50);
        expect(scores.bestPractices).toBeGreaterThan(50);
      });
    });

    describe('scoreKubernetesManifest', () => {
      it('should score Kubernetes manifest', () => {
        const manifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  labels:
    app: myapp
  annotations:
    prometheus.io/scrape: "true"
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        resources:
          requests:
            memory: "64Mi"
            cpu: "250m"
          limits:
            memory: "128Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
        readinessProbe:
          httpGet:
            path: /ready
        securityContext:
          runAsNonRoot: true
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
`;
        const scores = scoreKubernetesManifest(manifest);

        expect(scores.resources).toBeGreaterThan(50);
        expect(scores.security).toBeGreaterThan(50);
        expect(scores.reliability).toBeGreaterThan(50);
        expect(scores.observability).toBeGreaterThan(0);
      });
    });

    describe('scoreHelmChart', () => {
      it('should score Helm chart content', () => {
        const helmChart = `
apiVersion: v2
name: myapp
version: 1.0.0
description: A Helm chart for myapp
type: application
appVersion: 1.0.0

# Default values for myapp
replicaCount: 3
image:
  repository: myapp
  tag: latest
service:
  type: ClusterIP
  port: 8080
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 250m
    memory: 256Mi

# templates content with templating
{{- if .Values.ingress.enabled }}
{{- range .Values.ingress.hosts }}
  - host: {{ .host | quote }}
{{- end }}
{{- end }}
securityContext:
  runAsNonRoot: true
podSecurityContext: {}
livenessProbe:
  httpGet:
    path: /health
readinessProbe:
  httpGet:
    path: /ready
nodeSelector: {}
serviceAccount:
  create: true
`;
        const scores = scoreHelmChart(helmChart);

        expect(scores.chartStructure).toBeGreaterThan(80); // 90: all structure elements present
        expect(scores.templating).toBeGreaterThan(40); // 65: if, range, end blocks
        expect(scores.values).toBeGreaterThan(80); // 100: all values elements
        expect(scores.bestPractices).toBeGreaterThan(80); // 90: security, probes, scheduling
      });

      it('should give higher scores for well-structured charts', () => {
        const optimizedChart = `
apiVersion: v2
name: myapp
version: 1.0.0
description: A production-ready Helm chart
type: application
appVersion: 1.0.0

# -- Default values for myapp
replicaCount: 3
image:
  tag: 1.0.0
service:
  port: 8080
resources: {}

{{- include "myapp.labels" . | nindent 4 }}
{{- with .Values.nodeSelector }}
{{- toYaml . | nindent 8 }}
{{- end }}
securityContext: {}
podSecurityContext: {}
livenessProbe: {}
readinessProbe: {}
serviceAccount: {}
`;
        const scores = scoreHelmChart(optimizedChart);

        expect(scores.chartStructure).toBeGreaterThan(80);
        expect(scores.templating).toBeGreaterThan(40);
        expect(scores.values).toBeGreaterThan(60);
        expect(scores.bestPractices).toBeGreaterThan(30);
      });
    });

    describe('scoreACAManifest', () => {
      it('should score Azure Container Apps manifest in JSON format', () => {
        const acaManifest = `
{
  "type": "Microsoft.App/containerApps",
  "apiVersion": "2022-03-01",
  "name": "myapp",
  "properties": {
    "configuration": {
      "ingress": {
        "external": true,
        "targetPort": 8080,
        "allowInsecure": false,
        "transport": "http2"
      },
      "secrets": [
        {
          "name": "connection-string",
          "value": "encrypted-value"
        }
      ],
      "registries": [
        {
          "server": "myregistry.azurecr.io",
          "identity": "system"
        }
      ],
      "environmentVariables": [
        {
          "name": "NODE_ENV",
          "value": "production"
        }
      ]
    },
    "template": {
      "containers": [
        {
          "name": "myapp",
          "image": "myapp:latest",
          "resources": {
            "cpu": "0.5",
            "memory": "1Gi"
          }
        }
      ],
      "scale": {
        "minReplicas": 1,
        "maxReplicas": 10,
        "rules": [
          {
            "name": "http-requests",
            "http": {
              "metadata": {
                "concurrentRequests": "100"
              }
            }
          }
        ]
      }
    },
    "managedIdentity": {
      "type": "SystemAssigned"
    },
    "activeRevisionsMode": "single"
  }
}
`;
        const scores = scoreACAManifest(acaManifest);

        expect(scores.structure).toBeGreaterThan(25); // 30: only Microsoft.App/containerApps matches
        expect(scores.configuration).toBe(0); // 0: JSON format doesn't match YAML patterns
        expect(scores.scaling).toBe(0); // 0: JSON format doesn't match YAML patterns
        expect(scores.security).toBeGreaterThan(10); // 15: only allowInsecure not true matches
      });

      it('should score ACA manifest in YAML format', () => {
        const acaYaml = `
type: Microsoft.App/containerApps
apiVersion: 2022-03-01
name: myapp
properties:
  configuration:
    ingress:
      external: true
      targetPort: 8080
      allowInsecure: false
      transport: http2
    secrets:
      - name: connection-string
        value: encrypted-value
    registries:
      - server: myregistry.azurecr.io
        identity: system
    environmentVariables:
      - name: NODE_ENV
        value: production
  template:
    containers:
      - name: myapp
        image: myapp:1.0.0
    scale:
      minReplicas: 1
      maxReplicas: 10
      rules:
        - name: http-requests
          http:
            metadata:
              concurrentRequests: "100"
  managedIdentity:
    type: SystemAssigned
  activeRevisionsMode: single
`;
        const scores = scoreACAManifest(acaYaml);

        expect(scores.structure).toBeGreaterThan(75); // 100: all structure elements match
        expect(scores.configuration).toBeGreaterThan(75); // 100: ingress, registries, secrets, env vars
        expect(scores.scaling).toBeGreaterThan(75); // 100: scale, min/max, rules, http
        expect(scores.security).toBeGreaterThan(65); // 90: allowInsecure false, managedId, single mode, http2
      });
    });

    describe('scoreBaseImageRecommendation', () => {
      it('should score base image recommendations', () => {
        const recommendation = `
Recommended base images for Node.js application:

1. Minimal image (production): node:18-alpine
   - Size: ~45MB compressed
   - Security: Alpine Linux with minimal attack surface
   - No known vulnerabilities in scan
   - Official image maintained by Docker
   - LTS support and stable release
   - Updated and supported by community

2. Standard image (balanced): node:18-slim
   - Size: ~120MB compressed
   - Security: Debian slim variant, regularly updated
   - Official image with LTS support
   - Includes common utilities
   - Maintained and optimized builds

3. Security-hardened: gcr.io/distroless/nodejs18-debian11
   - Size: ~35MB compressed
   - Security: Distroless image, no shell or package manager
   - Vulnerability scan: clean
   - Minimal attack surface, production optimized
   - Official and signed images with LTS support
`;
        const scores = scoreBaseImageRecommendation(recommendation);

        expect(scores.specificity).toBeGreaterThan(50); // 55: alpine/slim/distroless, no generic
        expect(scores.security).toBeGreaterThan(70); // 85: distroless, alpine, vuln scan
        expect(scores.optimization).toBeGreaterThan(35); // 40: alpine/slim/distroless, compressed
        expect(scores.maintenance).toBeGreaterThan(80); // 100: LTS, stable, maintained, updated, supported
      });

      it('should penalize generic or deprecated tags', () => {
        const badRecommendation = `
Use these generic images:
- ubuntu:latest (large, generic)
- centos:latest (deprecated)
- node:latest (unstable tag)
`;
        const scores = scoreBaseImageRecommendation(badRecommendation);

        expect(scores.specificity).toBeLessThan(30);
        expect(scores.security).toBeLessThan(40);
        expect(scores.maintenance).toBeLessThan(50);
      });
    });

    describe('scoreContent', () => {
      it('should detect and score Dockerfile', () => {
        const dockerfile = 'FROM alpine\nRUN apk update';
        const result = scoreContent(dockerfile);

        expect(typeof result).toBe('object');
        expect(result).toHaveProperty('security');
        expect(result).toHaveProperty('caching');
      });

      it('should detect and score Kubernetes manifest', () => {
        const k8s = 'apiVersion: v1\nkind: Pod\n';
        const result = scoreContent(k8s);

        expect(typeof result).toBe('object');
        expect(result).toHaveProperty('resources');
        expect(result).toHaveProperty('reliability');
      });

      it('should return default score for unknown content', () => {
        const unknown = 'Some random text';
        const result = scoreContent(unknown);

        expect(result).toBe(50);
      });
    });
  });
});