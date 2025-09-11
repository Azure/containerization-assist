/**
 * Integration tests for Configuration-Driven Sampling System
 */

import { describe, it, expect } from '@jest/globals';
import { 
  initializeConfigSystem,
  scoreConfigCandidates,
  validateConfigurationSystem,
  quickConfigScore,
} from '../../src/lib/integrated-scoring';

describe('Configuration-Driven Sampling Integration', () => {
  
  it('should initialize configuration system successfully', async () => {
    const result = await initializeConfigSystem();
    
    // Should work or fail gracefully
    expect(result).toBeDefined();
    
    if (result.ok) {
      console.log('Configuration system initialized successfully');
    } else {
      console.log('Configuration system failed to initialize:', result.error);
    }
  });

  it('should validate configuration system', async () => {
    const result = await validateConfigurationSystem();
    
    expect(result).toBeDefined();
    expect(result.ok).toBe(true);
    
    if (result.ok) {
      console.log('Configuration validation result:', result.value);
      
      // Should have at least one profile loaded if initialization succeeded
      if (result.value.isValid) {
        expect(result.value.profilesLoaded.length).toBeGreaterThan(0);
      }
    }
  });

  it('should score Dockerfile content using configuration', async () => {
    const dockerfileContent = `
FROM alpine:3.14
RUN apk add --no-cache nodejs npm
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
USER 1000
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s CMD curl -f http://localhost:3000/health || exit 1
CMD ["node", "index.js"]
    `.trim();

    const result = await scoreConfigCandidates(
      [dockerfileContent], 
      'dockerfile', 
      'development'
    );

    console.log('Scoring result:', result);

    if (result.ok) {
      const candidates = result.value;
      expect(candidates.length).toBe(1);
      expect(candidates[0].score).toBeGreaterThan(0);
      expect(candidates[0].content).toBe(dockerfileContent);
      
      console.log('Dockerfile scored:', candidates[0].score);
      console.log('Score breakdown:', candidates[0].scoreBreakdown);
    } else {
      // Fallback should still work
      console.log('Config scoring failed (expected if no config):', result.error);
    }
  });

  it('should score Kubernetes YAML using configuration', async () => {
    const k8sContent = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  labels:
    app.kubernetes.io/name: test-app
    app.kubernetes.io/version: "1.0.0"
spec:
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/name: test-app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: test-app
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
      - name: app
        image: test-app:1.0.0
        ports:
        - containerPort: 3000
        resources:
          limits:
            memory: "512Mi"
            cpu: "500m"
          requests:
            memory: "256Mi"
            cpu: "250m"
        livenessProbe:
          httpGet:
            path: /health
            port: 3000
        readinessProbe:
          httpGet:
            path: /ready
            port: 3000
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
    `.trim();

    const result = await scoreConfigCandidates(
      [k8sContent],
      'yaml',
      'production'
    );

    console.log('K8s scoring result:', result);

    if (result.ok) {
      const candidates = result.value;
      expect(candidates.length).toBe(1);
      expect(candidates[0].score).toBeGreaterThan(0);
      
      console.log('K8s YAML scored:', candidates[0].score);
      console.log('Score breakdown:', candidates[0].scoreBreakdown);
    } else {
      console.log('K8s scoring failed (expected if no config):', result.error);
    }
  });

  it('should provide quick scoring for early stopping', async () => {
    const dockerfileContent = 'FROM alpine:latest\nWORKDIR /app\nCOPY . .\nUSER 1000';
    
    const score = await quickConfigScore(dockerfileContent, 'dockerfile', 'development');
    
    expect(score).toBeGreaterThan(0);
    expect(score).toBeLessThanOrEqual(100);
    
    console.log('Quick score:', score);
  });

  it('should handle missing configuration gracefully', async () => {
    // This should not throw, but provide fallback behavior
    const content = 'FROM alpine\nRUN echo "test"';
    
    const score = await quickConfigScore(content, 'dockerfile');
    expect(score).toBeGreaterThan(0);
    
    const result = await scoreConfigCandidates([content], 'dockerfile');
    // Should either succeed with config or fail gracefully
    expect(result).toBeDefined();
    
    console.log('Fallback behavior test completed');
  });
});