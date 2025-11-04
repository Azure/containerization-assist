/**
 * Tests for validation helper functions
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { promises as fs } from 'node:fs';
import * as path from 'node:path';
import * as os from 'node:os';
import {
  parseImageName,
  validatePathOrFail,
  validateImageTag,
  createPathValidator,
  mergeRegistryWithImage,
  type ParsedImageName,
} from '@/lib/validation-helpers';

describe('validation-helpers', () => {
  describe('parseImageName', () => {
    describe('valid image names', () => {
      it('should parse simple image with tag', () => {
        const result = parseImageName('node:20-alpine');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.repository).toBe('node');
          expect(result.value.tag).toBe('20-alpine');
          expect(result.value.registry).toBeUndefined();
          expect(result.value.fullName).toBe('node:20-alpine');
        }
      });

      it('should parse image with org/repo format', () => {
        const result = parseImageName('myorg/myapp:v1.0.0');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.repository).toBe('myorg/myapp');
          expect(result.value.tag).toBe('v1.0.0');
          expect(result.value.registry).toBeUndefined();
          expect(result.value.fullName).toBe('myorg/myapp:v1.0.0');
        }
      });

      it('should parse image with registry', () => {
        const result = parseImageName('docker.io/library/node:20');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.registry).toBe('docker.io');
          expect(result.value.repository).toBe('library/node');
          expect(result.value.tag).toBe('20');
          expect(result.value.fullName).toBe('docker.io/library/node:20');
        }
      });

      it('should parse image with private registry', () => {
        const result = parseImageName('registry.example.com/team/app:latest');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.registry).toBe('registry.example.com');
          expect(result.value.repository).toBe('team/app');
          expect(result.value.tag).toBe('latest');
          expect(result.value.fullName).toBe('registry.example.com/team/app:latest');
        }
      });

      it('should default to latest tag when tag is omitted', () => {
        const result = parseImageName('node');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.repository).toBe('node');
          expect(result.value.tag).toBe('latest');
          expect(result.value.registry).toBeUndefined();
        }
      });

      it('should default to latest tag for org/repo without tag', () => {
        const result = parseImageName('myorg/myapp');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.repository).toBe('myorg/myapp');
          expect(result.value.tag).toBe('latest');
          expect(result.value.registry).toBeUndefined();
        }
      });

      it('should handle registry with port', () => {
        const result = parseImageName('localhost:5000/myapp:dev');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.registry).toBe('localhost:5000');
          expect(result.value.repository).toBe('myapp');
          expect(result.value.tag).toBe('dev');
        }
      });

      it('should handle registry with port and no tag', () => {
        const result = parseImageName('localhost:5000/myapp');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.registry).toBe('localhost:5000');
          expect(result.value.repository).toBe('myapp');
          expect(result.value.tag).toBe('latest');
        }
      });

      it('should handle complex registry URLs', () => {
        const result = parseImageName('gcr.io/my-project/my-app:v2.0.0-rc1');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.registry).toBe('gcr.io');
          expect(result.value.repository).toBe('my-project/my-app');
          expect(result.value.tag).toBe('v2.0.0-rc1');
        }
      });

      it('should handle tags with multiple dots and hyphens', () => {
        const result = parseImageName('node:18.17.0-alpine3.18');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.repository).toBe('node');
          expect(result.value.tag).toBe('18.17.0-alpine3.18');
        }
      });

      it('should handle digest-style tags', () => {
        const result = parseImageName('node:sha256-abc123');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.repository).toBe('node');
          expect(result.value.tag).toBe('sha256-abc123');
        }
      });
    });

    describe('invalid image names', () => {
      it('should reject empty image name', () => {
        const result = parseImageName('');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('empty');
        }
      });

      it('should reject whitespace-only image name', () => {
        const result = parseImageName('   ');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('empty');
        }
      });

      it('should reject image name with invalid tag characters', () => {
        const result = parseImageName('node:invalid tag!');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('tag');
        }
      });

      it('should accept uppercase in tag (Docker allows it)', () => {
        const result = parseImageName('node:LatestVersion');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.repository).toBe('node');
          expect(result.value.tag).toBe('LatestVersion');
        }
      });

      it('should reject tag that is too long', () => {
        const longTag = 'a'.repeat(129);
        const result = parseImageName(`node:${longTag}`);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('128');
        }
      });
    });
  });

  describe('validatePathOrFail', () => {
    let tmpDir: string;

    beforeEach(async () => {
      // Create temporary directory for tests
      tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'validation-helpers-test-'));
    });

    afterEach(async () => {
      // Clean up temporary directory
      await fs.rm(tmpDir, { recursive: true, force: true });
    });

    it('should validate existing directory', async () => {
      const result = await validatePathOrFail(tmpDir, {
        mustExist: true,
        mustBeDirectory: true,
      });
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(path.isAbsolute(result.value)).toBe(true);
      }
    });

    it('should validate existing file', async () => {
      const filePath = path.join(tmpDir, 'test.txt');
      await fs.writeFile(filePath, 'test content');

      const result = await validatePathOrFail(filePath, {
        mustExist: true,
        mustBeFile: true,
      });
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(path.isAbsolute(result.value)).toBe(true);
      }
    });

    it('should reject non-existent path when mustExist is true', async () => {
      const nonExistentPath = path.join(tmpDir, 'does-not-exist');
      const result = await validatePathOrFail(nonExistentPath, {
        mustExist: true,
      });
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('does not exist');
      }
    });

    it('should reject file when directory is required', async () => {
      const filePath = path.join(tmpDir, 'file.txt');
      await fs.writeFile(filePath, 'content');

      const result = await validatePathOrFail(filePath, {
        mustExist: true,
        mustBeDirectory: true,
      });
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not a directory');
      }
    });

    it('should reject directory when file is required', async () => {
      const result = await validatePathOrFail(tmpDir, {
        mustExist: true,
        mustBeFile: true,
      });
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not a file');
      }
    });

    it('should resolve relative paths to absolute', async () => {
      const result = await validatePathOrFail('.');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(path.isAbsolute(result.value)).toBe(true);
      }
    });

    it('should accept path without existence check', async () => {
      const nonExistentPath = path.join(tmpDir, 'future-directory');
      const result = await validatePathOrFail(nonExistentPath);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(path.isAbsolute(result.value)).toBe(true);
      }
    });
  });

  describe('validateImageTag', () => {
    it('should validate simple tag', () => {
      const result = validateImageTag('latest');
      expect(result.ok).toBe(true);
    });

    it('should validate version tag', () => {
      const result = validateImageTag('v1.0.0');
      expect(result.ok).toBe(true);
    });

    it('should validate tag with hyphens and underscores', () => {
      const result = validateImageTag('1.0.0-alpine_test');
      expect(result.ok).toBe(true);
    });

    it('should validate tag with dots', () => {
      const result = validateImageTag('18.17.0');
      expect(result.ok).toBe(true);
    });

    it('should reject empty tag', () => {
      const result = validateImageTag('');
      expect(result.ok).toBe(false);
    });

    it('should reject tag with spaces', () => {
      const result = validateImageTag('latest version');
      expect(result.ok).toBe(false);
    });

    it('should reject tag with special characters', () => {
      const result = validateImageTag('v1.0.0!');
      expect(result.ok).toBe(false);
    });

    it('should reject tag that is too long', () => {
      const longTag = 'a'.repeat(129);
      const result = validateImageTag(longTag);
      expect(result.ok).toBe(false);
    });

    it('should accept uppercase letters (Docker allows both)', () => {
      const result = validateImageTag('LatestVersion');
      expect(result.ok).toBe(true);
    });

    it('should reject tag starting with period', () => {
      const result = validateImageTag('.latest');
      expect(result.ok).toBe(false);
    });

    it('should reject tag starting with hyphen', () => {
      const result = validateImageTag('-latest');
      expect(result.ok).toBe(false);
    });
  });

  describe('createPathValidator', () => {
    let tmpDir: string;

    beforeEach(async () => {
      tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'validation-helpers-factory-test-'));
    });

    afterEach(async () => {
      await fs.rm(tmpDir, { recursive: true, force: true });
    });

    it('should create reusable validator function', async () => {
      const validateDirectory = createPathValidator({
        mustExist: true,
        mustBeDirectory: true,
      });

      // Use the validator multiple times
      const result1 = await validateDirectory(tmpDir);
      expect(result1.ok).toBe(true);

      const subDir = path.join(tmpDir, 'subdir');
      await fs.mkdir(subDir);
      const result2 = await validateDirectory(subDir);
      expect(result2.ok).toBe(true);
    });

    it('should reject invalid paths with validator', async () => {
      const validateDirectory = createPathValidator({
        mustExist: true,
        mustBeDirectory: true,
      });

      const filePath = path.join(tmpDir, 'file.txt');
      await fs.writeFile(filePath, 'content');

      const result = await validateDirectory(filePath);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not a directory');
      }
    });

    it('should create validator with custom options', async () => {
      const validateFile = createPathValidator({
        mustExist: true,
        mustBeFile: true,
      });

      const filePath = path.join(tmpDir, 'test.txt');
      await fs.writeFile(filePath, 'content');

      const result = await validateFile(filePath);
      expect(result.ok).toBe(true);
    });
  });

  describe('edge cases', () => {
    it('parseImageName should handle null input gracefully', () => {
      // @ts-expect-error - Testing runtime behavior with invalid input
      const result = parseImageName(null);
      expect(result.ok).toBe(false);
    });

    it('parseImageName should handle undefined input gracefully', () => {
      // @ts-expect-error - Testing runtime behavior with invalid input
      const result = parseImageName(undefined);
      expect(result.ok).toBe(false);
    });

    it('validateImageTag should handle null input gracefully', () => {
      // @ts-expect-error - Testing runtime behavior with invalid input
      const result = validateImageTag(null);
      expect(result.ok).toBe(false);
    });

    it('parseImageName should handle very long repository names', () => {
      const longRepo = 'a'.repeat(200);
      const result = parseImageName(`${longRepo}:latest`);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.repository).toBe(longRepo);
        expect(result.value.tag).toBe('latest');
      }
    });
  });

  describe('mergeRegistryWithImage', () => {
    describe('avoiding double-prefixing', () => {
      it('should not double-prefix when image already has exact target registry', () => {
        const parsed: ParsedImageName = {
          registry: 'gcr.io',
          repository: 'proj/app',
          tag: 'v1',
          fullName: 'gcr.io/proj/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj');
        expect(result).toBe('gcr.io/proj/app');
      });

      it('should not double-prefix when image starts with target registry path', () => {
        const parsed: ParsedImageName = {
          registry: 'gcr.io',
          repository: 'proj/subproj/app',
          tag: 'v1',
          fullName: 'gcr.io/proj/subproj/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj');
        expect(result).toBe('gcr.io/proj/subproj/app');
      });

      it('should not double-prefix localhost registry with port', () => {
        const parsed: ParsedImageName = {
          registry: 'localhost:5000',
          repository: 'app',
          tag: 'v1',
          fullName: 'localhost:5000/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'localhost:5000');
        expect(result).toBe('localhost:5000/app');
      });
    });

    describe('handling registry hostname matches with different paths', () => {
      it('should replace project path when registry hostname matches', () => {
        const parsed: ParsedImageName = {
          registry: 'gcr.io',
          repository: 'proj1/app',
          tag: 'v1',
          fullName: 'gcr.io/proj1/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj2');
        expect(result).toBe('gcr.io/proj2/proj1/app');
      });

      it('should replace project path for Azure Container Registry', () => {
        const parsed: ParsedImageName = {
          registry: 'myregistry.azurecr.io',
          repository: 'app',
          tag: 'v1',
          fullName: 'myregistry.azurecr.io/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'myregistry.azurecr.io/proj');
        expect(result).toBe('myregistry.azurecr.io/proj/app');
      });

      it('should handle nested paths in ACR correctly', () => {
        const parsed: ParsedImageName = {
          registry: 'myregistry.azurecr.io',
          repository: 'team/app',
          tag: 'v1',
          fullName: 'myregistry.azurecr.io/team/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'myregistry.azurecr.io/proj');
        expect(result).toBe('myregistry.azurecr.io/proj/team/app');
      });
    });

    describe('replacing different registries', () => {
      it('should replace docker.io with custom registry', () => {
        const parsed: ParsedImageName = {
          registry: 'docker.io',
          repository: 'library/nginx',
          tag: 'latest',
          fullName: 'docker.io/library/nginx:latest',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj');
        expect(result).toBe('gcr.io/proj/library/nginx');
      });

      it('should replace localhost registry with cloud registry', () => {
        const parsed: ParsedImageName = {
          registry: 'localhost:5000',
          repository: 'myapp',
          tag: 'dev',
          fullName: 'localhost:5000/myapp:dev',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/production');
        expect(result).toBe('gcr.io/production/myapp');
      });

      it('should replace GCR with ACR', () => {
        const parsed: ParsedImageName = {
          registry: 'gcr.io',
          repository: 'old-proj/app',
          tag: 'v1',
          fullName: 'gcr.io/old-proj/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'myregistry.azurecr.io');
        expect(result).toBe('myregistry.azurecr.io/old-proj/app');
      });
    });

    describe('handling images without explicit registry', () => {
      it('should add registry to simple image name', () => {
        const parsed: ParsedImageName = {
          repository: 'myapp',
          tag: 'latest',
          fullName: 'myapp:latest',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj');
        expect(result).toBe('gcr.io/proj/myapp');
      });

      it('should preserve namespace when adding registry', () => {
        const parsed: ParsedImageName = {
          repository: 'myorg/myapp',
          tag: 'v1',
          fullName: 'myorg/myapp:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj');
        expect(result).toBe('gcr.io/proj/myorg/myapp');
      });

      it('should handle deep namespace paths', () => {
        const parsed: ParsedImageName = {
          repository: 'team/subteam/app',
          tag: 'v1',
          fullName: 'team/subteam/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'myregistry.azurecr.io');
        expect(result).toBe('myregistry.azurecr.io/team/subteam/app');
      });
    });

    describe('handling protocol and trailing slashes', () => {
      it('should strip https:// protocol from target registry', () => {
        const parsed: ParsedImageName = {
          repository: 'app',
          tag: 'v1',
          fullName: 'app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'https://gcr.io/proj');
        expect(result).toBe('gcr.io/proj/app');
      });

      it('should strip http:// protocol from target registry', () => {
        const parsed: ParsedImageName = {
          repository: 'app',
          tag: 'v1',
          fullName: 'app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'http://localhost:5000');
        expect(result).toBe('localhost:5000/app');
      });

      it('should strip trailing slash from target registry', () => {
        const parsed: ParsedImageName = {
          repository: 'app',
          tag: 'v1',
          fullName: 'app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj/');
        expect(result).toBe('gcr.io/proj/app');
      });

      it('should handle both protocol and trailing slash', () => {
        const parsed: ParsedImageName = {
          repository: 'app',
          tag: 'v1',
          fullName: 'app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'https://gcr.io/proj/');
        expect(result).toBe('gcr.io/proj/app');
      });
    });

    describe('edge cases', () => {
      it('should handle registry-like namespace correctly', () => {
        // Namespace "registry.internal" looks like a registry but isn't parsed as one
        const parsed: ParsedImageName = {
          repository: 'registry.internal/app',
          tag: 'v1',
          fullName: 'registry.internal/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj');
        // First part looks like registry, should be stripped
        expect(result).toBe('gcr.io/proj/app');
      });

      it('should preserve regular namespace that does not look like registry', () => {
        const parsed: ParsedImageName = {
          repository: 'myteam/app',
          tag: 'v1',
          fullName: 'myteam/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj');
        expect(result).toBe('gcr.io/proj/myteam/app');
      });

      it('should handle image with port-like pattern in name', () => {
        const parsed: ParsedImageName = {
          repository: 'app-v2',
          tag: 'latest',
          fullName: 'app-v2:latest',
        };
        const result = mergeRegistryWithImage(parsed, 'localhost:5000');
        expect(result).toBe('localhost:5000/app-v2');
      });

      it('should handle empty repository segments correctly', () => {
        const parsed: ParsedImageName = {
          registry: 'gcr.io',
          repository: 'proj/app',
          tag: 'v1',
          fullName: 'gcr.io/proj/app:v1',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/proj');
        expect(result).toBe('gcr.io/proj/app');
      });
    });

    describe('real-world scenarios from prepare-cluster', () => {
      it('should handle local registry to local registry (development)', () => {
        const parsed: ParsedImageName = {
          registry: 'localhost:5000',
          repository: 'myapp',
          tag: 'latest',
          fullName: 'localhost:5000/myapp:latest',
        };
        const result = mergeRegistryWithImage(parsed, 'localhost:5000');
        expect(result).toBe('localhost:5000/myapp');
      });

      it('should handle local registry to remote ACR (production)', () => {
        const parsed: ParsedImageName = {
          registry: 'localhost:5000',
          repository: 'myapp',
          tag: 'v1.0.0',
          fullName: 'localhost:5000/myapp:v1.0.0',
        };
        const result = mergeRegistryWithImage(parsed, 'myregistry.azurecr.io/production');
        expect(result).toBe('myregistry.azurecr.io/production/myapp');
      });

      it('should handle Docker Hub to GCR migration', () => {
        const parsed: ParsedImageName = {
          repository: 'myorg/myapp',
          tag: 'v2.0.0',
          fullName: 'myorg/myapp:v2.0.0',
        };
        const result = mergeRegistryWithImage(parsed, 'gcr.io/my-project');
        expect(result).toBe('gcr.io/my-project/myorg/myapp');
      });

      it('should handle ECR with region in URL', () => {
        const parsed: ParsedImageName = {
          repository: 'myapp',
          tag: 'latest',
          fullName: 'myapp:latest',
        };
        const result = mergeRegistryWithImage(parsed, '123456789.dkr.ecr.us-east-1.amazonaws.com');
        expect(result).toBe('123456789.dkr.ecr.us-east-1.amazonaws.com/myapp');
      });
    });
  });
});
