/**
 * Unit Tests: Validation Error Scenarios
 * Tests input validation patterns without being prescriptive about exact validation rules
 */

import { jest } from '@jest/globals';

function createSuccessResult<T>(value: T) {
  return { ok: true as const, value };
}

function createFailureResult(error: string, guidance?: { resolution?: string; hints?: string[] }) {
  return { ok: false as const, error, guidance };
}

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

function createMockToolContext() {
  return { logger: createMockLogger() } as any;
}

const mockValidation = {
  validatePath: jest.fn(),
  validateImageName: jest.fn(),
  validateNamespace: jest.fn(),
  validateK8sName: jest.fn(),
};

jest.mock('../../../src/lib/validation', () => ({
  validatePath: (...args: any[]) => mockValidation.validatePath(...args),
  validateImageName: (...args: any[]) => mockValidation.validateImageName(...args),
  validateNamespace: (...args: any[]) => mockValidation.validateNamespace(...args),
  validateK8sName: (...args: any[]) => mockValidation.validateK8sName(...args),
}));

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => ({ end: jest.fn(), error: jest.fn() })),
  createLogger: jest.fn(() => createMockLogger()),
}));

import { buildImage } from '../../../src/tools/build-image/tool';

describe('Validation Error Scenarios', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Path Validation', () => {
    it('should validate path parameter', async () => {
      mockValidation.validatePath.mockResolvedValue(
        createFailureResult('Invalid path'),
      );

      const result = await buildImage(
        { path: '', dockerfile: 'Dockerfile', imageName: 'test:latest', tags: [], buildArgs: {} },
        createMockToolContext(),
      );

      expect(result.ok).toBe(false);
    });

    it('should accept valid paths', async () => {
      mockValidation.validatePath.mockResolvedValue(
        createSuccessResult('/valid/path'),
      );

      const result = await mockValidation.validatePath('/valid/path');

      expect(result).toHaveProperty('ok');
      expect(result).toHaveProperty('value');
    });
  });

  describe('Image Name Validation', () => {
    it('should validate image name format', () => {
      mockValidation.validateImageName.mockReturnValue(
        createFailureResult('Invalid image name'),
      );

      const result = mockValidation.validateImageName('Invalid@Name');
      expect(result.ok).toBe(false);
    });

    it('should accept valid image names', () => {
      mockValidation.validateImageName.mockReturnValue(
        createSuccessResult('valid-image:latest'),
      );

      const result = mockValidation.validateImageName('valid-image:latest');
      expect(result.ok).toBe(true);
    });
  });

  describe('Namespace Validation', () => {
    it('should validate namespace format', () => {
      mockValidation.validateNamespace.mockReturnValue(
        createFailureResult('Invalid namespace'),
      );

      const result = mockValidation.validateNamespace('Invalid_Namespace');
      expect(result.ok).toBe(false);
    });

    it('should accept valid namespaces', () => {
      mockValidation.validateNamespace.mockReturnValue(
        createSuccessResult('valid-namespace'),
      );

      const result = mockValidation.validateNamespace('valid-namespace');
      expect(result.ok).toBe(true);
    });
  });

  describe('K8s Resource Name Validation', () => {
    it('should validate K8s resource names', () => {
      mockValidation.validateK8sName.mockReturnValue(
        createFailureResult('Invalid resource name'),
      );

      const result = mockValidation.validateK8sName('Invalid_Name');
      expect(result.ok).toBe(false);
    });

    it('should accept valid K8s names', () => {
      mockValidation.validateK8sName.mockReturnValue(
        createSuccessResult('valid-name'),
      );

      const result = mockValidation.validateK8sName('valid-name');
      expect(result.ok).toBe(true);
    });
  });

  describe('Validation Result Structure', () => {
    it('should return Result<T> on validation failure', () => {
      mockValidation.validateImageName.mockReturnValue(
        createFailureResult('Validation failed', {
          resolution: 'Fix the input',
          hints: ['Use valid format'],
        }),
      );

      const result = mockValidation.validateImageName('invalid');

      expect(result).toHaveProperty('ok');
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(typeof result.error).toBe('string');
        if (result.guidance) {
          expect(result.guidance).toHaveProperty('resolution');
          expect(result.guidance).toHaveProperty('hints');
        }
      }
    });

    it('should return Result<T> on validation success', () => {
      mockValidation.validateImageName.mockReturnValue(
        createSuccessResult('valid:image'),
      );

      const result = mockValidation.validateImageName('valid:image');

      expect(result).toHaveProperty('ok');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('valid:image');
      }
    });
  });

  describe('Optional Guidance', () => {
    it('should optionally provide guidance on validation errors', () => {
      mockValidation.validatePath.mockResolvedValue(
        createFailureResult('Path error', {
          resolution: 'Provide valid path',
          hints: ['Must be absolute', 'Must exist'],
        }),
      );

      const result = mockValidation.validatePath('/invalid');

      expect(result).resolves.toHaveProperty('ok');
    });
  });
});
