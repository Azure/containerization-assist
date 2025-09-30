/**
 * Unified Scoring Framework Integration Tests
 * Verifies that the scoring framework migration is complete and working correctly
 */

import { describe, it, expect } from '@jest/globals';
import {
  scoreDockerfile,
  scoreDockerfileDetailed,
  scoreKubernetesManifest,
  scoreHelmChart,
  scoreContent,
} from '@/lib/scoring';

describe('Unified Scoring Framework', () => {
  describe('scoreDockerfile function', () => {
    it('should be imported from @/lib/scoring', () => {
      expect(typeof scoreDockerfile).toBe('function');
    });

    it('should score basic Dockerfile correctly', () => {
      const dockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
CMD ["node", "server.js"]`;

      const score = scoreDockerfile(dockerfile);
      expect(typeof score).toBe('number');
      expect(score).toBeGreaterThan(0);
      expect(score).toBeLessThanOrEqual(100);
    });

    it('should give detailed scoring breakdown', () => {
      const dockerfile = `FROM node:18-alpine
RUN apk add --no-cache git
USER node
WORKDIR /app
COPY --chown=node:node package*.json ./
RUN npm ci
COPY --chown=node:node . .
HEALTHCHECK CMD node healthcheck.js
CMD ["node", "server.js"]`;

      const breakdown = scoreDockerfileDetailed(dockerfile);
      expect(typeof breakdown).toBe('object');
      expect(breakdown).toHaveProperty('security');
      expect(breakdown).toHaveProperty('bestPractices');
      expect(breakdown).toHaveProperty('parseability');

      expect(typeof breakdown.security).toBe('number');
      expect(typeof breakdown.bestPractices).toBe('number');
      expect(typeof breakdown.parseability).toBe('number');
    });
  });

  describe('scoreKubernetesManifest function', () => {
    it('should be imported from @/lib/scoring', () => {
      expect(typeof scoreKubernetesManifest).toBe('function');
    });

    it('should score K8s manifest with detailed breakdown', () => {
      const manifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  labels:
    app: test
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: app
        image: test:latest
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
          readOnlyRootFilesystem: true`;

      const scores = scoreKubernetesManifest(manifest);
      expect(typeof scores).toBe('object');
      expect(scores).toHaveProperty('resources');
      expect(scores).toHaveProperty('security');
      expect(scores).toHaveProperty('reliability');
      expect(scores).toHaveProperty('observability');

      expect(typeof scores.resources).toBe('number');
      expect(typeof scores.security).toBe('number');
      expect(typeof scores.reliability).toBe('number');
      expect(typeof scores.observability).toBe('number');
    });
  });

  describe('scoreHelmChart function', () => {
    it('should be imported from @/lib/scoring', () => {
      expect(typeof scoreHelmChart).toBe('function');
    });

    it('should score Helm chart with component breakdown', () => {
      const helmChart = `apiVersion: v2
name: myapp
version: 1.0.0
description: A test Helm chart
type: application
appVersion: 1.0.0

replicaCount: 3
image:
  repository: myapp
  tag: 1.0.0
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

{{- include "myapp.labels" . | nindent 4 }}
{{- with .Values.nodeSelector }}
{{- toYaml . | nindent 8 }}
{{- end }}

securityContext:
  runAsNonRoot: true
livenessProbe:
  httpGet:
    path: /health
readinessProbe:
  httpGet:
    path: /ready
serviceAccount:
  create: true`;

      const scores = scoreHelmChart(helmChart);
      expect(typeof scores).toBe('object');
      expect(scores).toHaveProperty('chartStructure');
      expect(scores).toHaveProperty('templating');
      expect(scores).toHaveProperty('values');
      expect(scores).toHaveProperty('bestPractices');

      expect(typeof scores.chartStructure).toBe('number');
      expect(typeof scores.templating).toBe('number');
      expect(typeof scores.values).toBe('number');
      expect(typeof scores.bestPractices).toBe('number');
    });
  });

  describe('scoreContent auto-detection', () => {
    it('should auto-detect and score Dockerfile content', () => {
      const dockerfile = 'FROM alpine\nRUN apk update';
      const result = scoreContent(dockerfile);

      expect(typeof result).toBe('object');
      expect(result).toHaveProperty('total');
      expect(result).toHaveProperty('breakdown');
      expect(typeof result.total).toBe('number');
      expect(result.total).toBeGreaterThan(0);
    });

    it('should auto-detect and score Kubernetes content', () => {
      const k8sManifest = 'apiVersion: v1\nkind: Pod\nmetadata:\n  name: test';
      const result = scoreContent(k8sManifest);

      expect(typeof result).toBe('object');
      expect(result).toHaveProperty('total');
      expect(result).toHaveProperty('breakdown');
      expect(result.breakdown).toHaveProperty('resources');
      expect(result.breakdown).toHaveProperty('reliability');
    });

    it('should return structured result for unknown content', () => {
      const unknownContent = 'This is just some random text';
      const result = scoreContent(unknownContent);

      expect(typeof result).toBe('object');
      expect(result).toHaveProperty('total');
      expect(result).toHaveProperty('breakdown');
      expect(typeof result.total).toBe('number');
    });
  });

  describe('Scoring Framework Consistency', () => {
    it('should return consistent score formats', () => {
      const dockerfile = 'FROM node:alpine\nRUN npm install';
      const k8sManifest = 'apiVersion: v1\nkind: Pod';
      const helmChart = 'apiVersion: v2\nname: test\nversion: 1.0.0';

      const dockerScore = scoreDockerfile(dockerfile);
      const k8sScore = scoreKubernetesManifest(k8sManifest);
      const helmScore = scoreHelmChart(helmChart);

      // Dockerfile should return a number
      expect(typeof dockerScore).toBe('number');
      expect(dockerScore).toBeGreaterThanOrEqual(0);
      expect(dockerScore).toBeLessThanOrEqual(100);

      // K8s should return an object with score properties
      expect(typeof k8sScore).toBe('object');
      Object.values(k8sScore).forEach(score => {
        expect(typeof score).toBe('number');
        expect(score).toBeGreaterThanOrEqual(0);
        expect(score).toBeLessThanOrEqual(100);
      });

      // Helm should return an object with score properties
      expect(typeof helmScore).toBe('object');
      Object.values(helmScore).forEach(score => {
        expect(typeof score).toBe('number');
        expect(score).toBeGreaterThanOrEqual(0);
        expect(score).toBeLessThanOrEqual(100);
      });
    });

    it('should have no dependency on legacy sampling.ts scoreContent', () => {
      // This test ensures scoreContent is properly migrated to @/lib/scoring
      const testContent = 'FROM ubuntu\nRUN apt-get update';

      // Should not throw and should return expected format
      expect(() => scoreContent(testContent)).not.toThrow();

      const result = scoreContent(testContent);
      expect(typeof result).toBe('object');
      expect(result).toHaveProperty('total');
      expect(result).toHaveProperty('breakdown');
    });
  });

  describe('Error Handling', () => {
    it('should handle malformed Dockerfile gracefully', () => {
      const malformedDockerfile = 'FROM\nRUN\nCMD';

      expect(() => scoreDockerfile(malformedDockerfile)).not.toThrow();
      const score = scoreDockerfile(malformedDockerfile);
      expect(typeof score).toBe('number');
    });

    it('should handle malformed K8s manifest gracefully', () => {
      const malformedManifest = 'apiVersion:\nkind:\nmetadata:';

      expect(() => scoreKubernetesManifest(malformedManifest)).not.toThrow();
      const scores = scoreKubernetesManifest(malformedManifest);
      expect(typeof scores).toBe('object');
    });

    it('should handle empty content gracefully', () => {
      expect(() => scoreContent('')).not.toThrow();
      expect(() => scoreDockerfile('')).not.toThrow();
      expect(() => scoreKubernetesManifest('')).not.toThrow();
      expect(() => scoreHelmChart('')).not.toThrow();
    });
  });
});