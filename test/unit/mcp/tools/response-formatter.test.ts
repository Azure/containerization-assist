import { Success, Failure } from '@types';
import {
  formatStandardResponse,
  detectK8sKind,
  responseFormatters,
  StandardToolResponse,
  DockerfileResponse,
  ManifestResponse,
  AnalysisResponse,
  ScanResponse,
  DeploymentResponse
} from '@mcp/tools/response-formatter';

describe('Response Formatter', () => {
  describe('formatStandardResponse', () => {
    it('should format successful result with data and sessionId', () => {
      const input = Success({ message: 'test' });
      const result = formatStandardResponse(input, 'session-123');

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({
          ok: true,
          sessionId: 'session-123',
          data: { message: 'test' },
          message: 'Operation completed successfully'
        });
      }
    });

    it('should format successful result without sessionId', () => {
      const input = Success('test data');
      const result = formatStandardResponse(input);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({
          ok: true,
          sessionId: undefined,
          data: 'test data',
          message: 'Operation completed successfully'
        });
      }
    });

    it('should format failed result', () => {
      const input = Failure('Something went wrong');
      const result = formatStandardResponse(input, 'session-123');

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Something went wrong');
      }
    });

    it('should handle null and undefined data', () => {
      const nullResult = formatStandardResponse(Success(null), 'session-123');
      const undefinedResult = formatStandardResponse(Success(undefined), 'session-123');

      expect(nullResult.ok).toBe(true);
      expect(undefinedResult.ok).toBe(true);
      
      if (nullResult.ok) {
        expect(nullResult.value.data).toBeNull();
      }
      if (undefinedResult.ok) {
        expect(undefinedResult.value.data).toBeUndefined();
      }
    });
  });

  describe('detectK8sKind', () => {
    it('should detect Deployment kind', () => {
      const yaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
`;
      expect(detectK8sKind(yaml)).toBe('Deployment');
    });

    it('should detect Service kind', () => {
      const yaml = `
apiVersion: v1
kind: Service
metadata:
  name: my-service
`;
      expect(detectK8sKind(yaml)).toBe('Service');
    });

    it('should handle kind with extra whitespace', () => {
      const yaml = `
kind:   ConfigMap   
metadata:
  name: config
`;
      expect(detectK8sKind(yaml)).toBe('ConfigMap');
    });

    it('should return Unknown for missing kind', () => {
      const yaml = `
apiVersion: v1
metadata:
  name: no-kind
`;
      expect(detectK8sKind(yaml)).toBe('Unknown');
    });

    it('should return Unknown for empty yaml', () => {
      expect(detectK8sKind('')).toBe('Unknown');
    });
  });

  describe('responseFormatters.dockerfile', () => {
    it('should format dockerfile response with sessionId', () => {
      const result = responseFormatters.dockerfile('FROM node:18\nWORKDIR /app', 'session-123');
      
      expect(result).toEqual({
        ok: true,
        sessionId: 'session-123',
        dockerfile: 'FROM node:18\nWORKDIR /app',
        path: '/app/Dockerfile'
      });
    });

    it('should format dockerfile response without sessionId', () => {
      const result = responseFormatters.dockerfile('FROM python:3.9');
      
      expect(result).toEqual({
        ok: true,
        sessionId: undefined,
        dockerfile: 'FROM python:3.9',
        path: '/app/Dockerfile'
      });
    });
  });

  describe('responseFormatters.manifest', () => {
    it('should format manifest response with detected kind', () => {
      const yaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
`;
      const result = responseFormatters.manifest(yaml, 'session-456');
      
      expect(result).toEqual({
        ok: true,
        sessionId: 'session-456',
        manifest: yaml,
        kind: 'Deployment'
      });
    });

    it('should format manifest response with Unknown kind', () => {
      const yaml = 'invalid yaml without kind';
      const result = responseFormatters.manifest(yaml);
      
      expect(result).toEqual({
        ok: true,
        sessionId: undefined,
        manifest: yaml,
        kind: 'Unknown'
      });
    });
  });

  describe('responseFormatters.analysis', () => {
    it('should format analysis response', () => {
      const result = responseFormatters.analysis(
        'React',
        'TypeScript',
        ['react', 'typescript', 'webpack'],
        ['Add error boundaries', 'Use React.memo'],
        'session-789'
      );
      
      expect(result).toEqual({
        ok: true,
        sessionId: 'session-789',
        analysis: {
          framework: 'React',
          language: 'TypeScript',
          dependencies: ['react', 'typescript', 'webpack'],
          recommendations: ['Add error boundaries', 'Use React.memo']
        }
      });
    });

    it('should format analysis response without sessionId', () => {
      const result = responseFormatters.analysis(
        'Express',
        'JavaScript',
        ['express', 'cors'],
        ['Add helmet for security']
      );
      
      expect(result.ok).toBe(true);
      expect(result.sessionId).toBeUndefined();
      expect(result.analysis.framework).toBe('Express');
    });
  });

  describe('responseFormatters.scan', () => {
    it('should format scan response with calculated total', () => {
      const vulnerabilities = { critical: 2, high: 5, medium: 10, low: 3 };
      const result = responseFormatters.scan(
        vulnerabilities,
        'Found 20 vulnerabilities',
        'session-scan'
      );
      
      expect(result).toEqual({
        ok: true,
        sessionId: 'session-scan',
        vulnerabilities: {
          critical: 2,
          high: 5,
          medium: 10,
          low: 3,
          total: 20
        },
        summary: 'Found 20 vulnerabilities'
      });
    });

    it('should handle zero vulnerabilities', () => {
      const vulnerabilities = { critical: 0, high: 0, medium: 0, low: 0 };
      const result = responseFormatters.scan(
        vulnerabilities,
        'No vulnerabilities found'
      );
      
      expect(result.vulnerabilities.total).toBe(0);
      expect(result.summary).toBe('No vulnerabilities found');
    });
  });

  describe('responseFormatters.deployment', () => {
    it('should format successful deployment response', () => {
      const result = responseFormatters.deployment(
        true,
        ['deployment/my-app', 'service/my-app'],
        'Running',
        'session-deploy'
      );
      
      expect(result).toEqual({
        ok: true,
        sessionId: 'session-deploy',
        deployed: true,
        resources: ['deployment/my-app', 'service/my-app'],
        status: 'Running'
      });
    });

    it('should format failed deployment response', () => {
      const result = responseFormatters.deployment(
        false,
        [],
        'Failed',
        'session-fail'
      );
      
      expect(result).toEqual({
        ok: true,
        sessionId: 'session-fail',
        deployed: false,
        resources: [],
        status: 'Failed'
      });
    });
  });

  describe('Type Safety', () => {
    it('should have correct TypeScript types', () => {
      // These tests verify TypeScript compilation and type safety
      const standardResponse: StandardToolResponse<string> = {
        ok: true,
        sessionId: 'test',
        data: 'test data',
        message: 'success'
      };
      
      const dockerfileResponse: DockerfileResponse = {
        ok: true,
        dockerfile: 'FROM node',
        path: '/Dockerfile'
      };
      
      const manifestResponse: ManifestResponse = {
        ok: true,
        manifest: 'kind: Pod',
        kind: 'Pod'
      };
      
      const analysisResponse: AnalysisResponse = {
        ok: true,
        analysis: {
          framework: 'React',
          language: 'TypeScript',
          dependencies: [],
          recommendations: []
        }
      };
      
      const scanResponse: ScanResponse = {
        ok: true,
        vulnerabilities: {
          critical: 0,
          high: 0,
          medium: 0,
          low: 0,
          total: 0
        },
        summary: 'Clean'
      };
      
      const deploymentResponse: DeploymentResponse = {
        ok: true,
        deployed: true,
        resources: [],
        status: 'Running'
      };

      // If this compiles, types are correct
      expect(standardResponse.ok).toBe(true);
      expect(dockerfileResponse.ok).toBe(true);
      expect(manifestResponse.ok).toBe(true);
      expect(analysisResponse.ok).toBe(true);
      expect(scanResponse.ok).toBe(true);
      expect(deploymentResponse.ok).toBe(true);
    });
  });
});