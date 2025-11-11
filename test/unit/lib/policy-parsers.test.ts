/**
 * Unit Tests: Policy Parsing Utilities
 *
 * Tests for semantic validation parsing functions.
 */

import { describe, expect, it } from '@jest/globals';
import {
  parseCPU,
  parseMemory,
  extractImageName,
  extractPorts,
  calculateResourceRatio,
  extractManifestImages,
  extractServicePorts,
} from '@/lib/policy-parsers';

describe('Policy Parsers', () => {
  describe('parseCPU', () => {
    it('should parse millicores format', () => {
      const result = parseCPU('1000m');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(1000);
      }
    });

    it('should parse decimal cores format', () => {
      const result = parseCPU('1.5');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(1500);
      }
    });

    it('should parse integer cores format', () => {
      const result = parseCPU('2');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(2000);
      }
    });

    it('should parse fractional cores', () => {
      const result = parseCPU('0.5');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(500);
      }
    });

    it('should handle invalid CPU values', () => {
      const result = parseCPU('invalid');
      expect(result.ok).toBe(false);
    });

    it('should handle negative values', () => {
      const result = parseCPU('-100m');
      expect(result.ok).toBe(false);
    });
  });

  describe('parseMemory', () => {
    it('should parse mebibytes format', () => {
      const result = parseMemory('512Mi');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(536870912); // 512 * 1024^2
      }
    });

    it('should parse gibibytes format', () => {
      const result = parseMemory('2Gi');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(2147483648); // 2 * 1024^3
      }
    });

    it('should parse megabytes format', () => {
      const result = parseMemory('512M');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(512000000); // 512 * 1000^2
      }
    });

    it('should parse gigabytes format', () => {
      const result = parseMemory('1G');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(1000000000); // 1 * 1000^3
      }
    });

    it('should parse kibibytes format', () => {
      const result = parseMemory('1024Ki');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(1048576); // 1024 * 1024
      }
    });

    it('should parse plain bytes', () => {
      const result = parseMemory('1024');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(1024);
      }
    });

    it('should handle invalid memory values', () => {
      const result = parseMemory('invalid');
      expect(result.ok).toBe(false);
    });

    it('should handle negative values', () => {
      const result = parseMemory('-512Mi');
      expect(result.ok).toBe(false);
    });
  });

  describe('extractImageName', () => {
    it('should extract simple FROM instruction', () => {
      const dockerfile = 'FROM node:18\nRUN npm install\n';
      const result = extractImageName(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('node:18');
      }
    });

    it('should extract FROM with alpine', () => {
      const dockerfile = 'FROM node:18-alpine\nRUN npm install\n';
      const result = extractImageName(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('node:18-alpine');
      }
    });

    it('should extract last FROM in multi-stage build', () => {
      const dockerfile = `FROM node:18 AS builder
RUN npm build
FROM nginx:alpine
COPY --from=builder /app /usr/share/nginx/html`;
      const result = extractImageName(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('nginx:alpine');
      }
    });

    it('should handle FROM with AS stage name', () => {
      const dockerfile = 'FROM node:18 AS build-stage\nRUN npm install\n';
      const result = extractImageName(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('node:18');
      }
    });

    it('should handle case-insensitive FROM', () => {
      const dockerfile = 'from node:18\nRUN npm install\n';
      const result = extractImageName(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('node:18');
      }
    });

    it('should fail on missing FROM instruction', () => {
      const dockerfile = 'RUN npm install\nCMD ["node", "app.js"]\n';
      const result = extractImageName(dockerfile);
      expect(result.ok).toBe(false);
    });
  });

  describe('extractPorts', () => {
    it('should extract single EXPOSE port', () => {
      const dockerfile = 'FROM node:18\nEXPOSE 3000\n';
      const result = extractPorts(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual([3000]);
      }
    });

    it('should extract multiple EXPOSE ports', () => {
      const dockerfile = 'FROM node:18\nEXPOSE 3000\nEXPOSE 8080\n';
      const result = extractPorts(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toContain(3000);
        expect(result.value).toContain(8080);
      }
    });

    it('should extract multiple ports in single EXPOSE', () => {
      const dockerfile = 'FROM node:18\nEXPOSE 3000 8080 9090\n';
      const result = extractPorts(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toContain(3000);
        expect(result.value).toContain(8080);
        expect(result.value).toContain(9090);
      }
    });

    it('should handle EXPOSE with protocol suffix', () => {
      const dockerfile = 'FROM node:18\nEXPOSE 3000/tcp\n';
      const result = extractPorts(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual([3000]);
      }
    });

    it('should handle case-insensitive EXPOSE', () => {
      const dockerfile = 'FROM node:18\nexpose 3000\n';
      const result = extractPorts(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual([3000]);
      }
    });

    it('should return empty array if no EXPOSE instructions', () => {
      const dockerfile = 'FROM node:18\nRUN npm install\n';
      const result = extractPorts(dockerfile);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual([]);
      }
    });
  });

  describe('calculateResourceRatio', () => {
    it('should calculate ratio correctly', () => {
      const result = calculateResourceRatio(4000, 500);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(8);
      }
    });

    it('should handle equal values', () => {
      const result = calculateResourceRatio(1000, 1000);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(1);
      }
    });

    it('should handle fractional ratios', () => {
      const result = calculateResourceRatio(1500, 1000);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(1.5);
      }
    });

    it('should fail on zero request', () => {
      const result = calculateResourceRatio(1000, 0);
      expect(result.ok).toBe(false);
    });

    it('should fail on negative request', () => {
      const result = calculateResourceRatio(1000, -500);
      expect(result.ok).toBe(false);
    });

    it('should fail when limit is less than request', () => {
      const result = calculateResourceRatio(500, 1000);
      expect(result.ok).toBe(false);
    });
  });

  describe('extractManifestImages', () => {
    it('should extract images from Deployment', () => {
      const manifest = {
        kind: 'Deployment',
        spec: {
          template: {
            spec: {
              containers: [
                { name: 'app', image: 'myapp:v1' },
                { name: 'sidecar', image: 'sidecar:latest' },
              ],
            },
          },
        },
      };
      const result = extractManifestImages(manifest);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual(['myapp:v1', 'sidecar:latest']);
      }
    });

    it('should extract images from Pod', () => {
      const manifest = {
        kind: 'Pod',
        spec: {
          containers: [{ name: 'app', image: 'myapp:v1' }],
        },
      };
      const result = extractManifestImages(manifest);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual(['myapp:v1']);
      }
    });

    it('should return empty array for manifest without containers', () => {
      const manifest = {
        kind: 'Service',
        spec: { ports: [] },
      };
      const result = extractManifestImages(manifest);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual([]);
      }
    });

    it('should handle invalid manifest', () => {
      const result = extractManifestImages('invalid');
      expect(result.ok).toBe(false);
    });
  });

  describe('extractServicePorts', () => {
    it('should extract target ports from Service', () => {
      const service = {
        kind: 'Service',
        spec: {
          ports: [
            { port: 80, targetPort: 3000 },
            { port: 443, targetPort: 8443 },
          ],
        },
      };
      const result = extractServicePorts(service);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual([3000, 8443]);
      }
    });

    it('should handle string target ports', () => {
      const service = {
        kind: 'Service',
        spec: {
          ports: [
            { port: 80, targetPort: '3000' },
            { port: 443, targetPort: 'https' }, // Named port (should be ignored)
          ],
        },
      };
      const result = extractServicePorts(service);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual([3000]);
      }
    });

    it('should return empty array for service without ports', () => {
      const service = {
        kind: 'Service',
        spec: {},
      };
      const result = extractServicePorts(service);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual([]);
      }
    });

    it('should handle invalid service', () => {
      const result = extractServicePorts('invalid');
      expect(result.ok).toBe(false);
    });

    it('should handle service without spec', () => {
      const result = extractServicePorts({});
      expect(result.ok).toBe(false);
    });
  });
});
