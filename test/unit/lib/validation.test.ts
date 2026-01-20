/**
 * Tests for validation helpers
 */

import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

import {
  validateDockerTag,
  validateEnvName,
  validateImageName,
  validateK8sName,
  validateNamespace,
  validatePath,
  validatePort,
} from '../../../src/lib/validation';

describe('validation helpers', () => {
  describe('validatePath', () => {
    let tempDir: string;
    let tempFile: string;

    beforeEach(async () => {
      // Create temporary directory and file for testing
      tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'validation-test-'));
      tempFile = path.join(tempDir, 'test-file.txt');
      await fs.writeFile(tempFile, 'test content');
    });

    afterEach(async () => {
      // Clean up temporary files
      await fs.rm(tempDir, { recursive: true, force: true });
    });

    describe('basic path resolution', () => {
      it('should convert relative paths to absolute paths', async () => {
        const result = await validatePath('./src');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(path.isAbsolute(result.value)).toBe(true);
        }
      });

      it('should preserve absolute paths', async () => {
        const absolutePath = '/usr/local/bin';
        const result = await validatePath(absolutePath);
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(absolutePath);
        }
      });

      it('should resolve paths relative to current working directory', async () => {
        const cwd = process.cwd();
        const result = await validatePath('./src');
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBe(path.join(cwd, 'src'));
        }
      });
    });

    describe('existence validation', () => {
      it('should validate that a path exists when mustExist is true', async () => {
        const result = await validatePath(tempDir, { mustExist: true });
        expect(result.ok).toBe(true);
      });

      it('should fail when path does not exist and mustExist is true', async () => {
        const result = await validatePath('/nonexistent/path', { mustExist: true });
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('does not exist');
          expect(result.guidance?.hint).toBeDefined();
          expect(result.guidance?.resolution).toBeDefined();
        }
      });

      it('should succeed for nonexistent paths when mustExist is not set', async () => {
        const result = await validatePath('/nonexistent/path');
        expect(result.ok).toBe(true);
      });
    });

    describe('directory validation', () => {
      it('should validate that a path is a directory', async () => {
        const result = await validatePath(tempDir, {
          mustExist: true,
          mustBeDirectory: true,
        });
        expect(result.ok).toBe(true);
      });

      it('should fail when path is a file but mustBeDirectory is true', async () => {
        const result = await validatePath(tempFile, {
          mustExist: true,
          mustBeDirectory: true,
        });
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('not a directory');
          expect(result.guidance?.hint).toContain('file');
        }
      });
    });

    describe('file validation', () => {
      it('should validate that a path is a file', async () => {
        const result = await validatePath(tempFile, {
          mustExist: true,
          mustBeFile: true,
        });
        expect(result.ok).toBe(true);
      });

      it('should fail when path is a directory but mustBeFile is true', async () => {
        const result = await validatePath(tempDir, {
          mustExist: true,
          mustBeFile: true,
        });
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('not a file');
          expect(result.guidance?.hint).toContain('directory');
        }
      });
    });

    describe('readable validation', () => {
      it('should validate that a path is readable', async () => {
        const result = await validatePath(tempFile, {
          readable: true,
        });
        expect(result.ok).toBe(true);
      });

      it('should fail when path is not readable', async () => {
        // Skip this test when running as root (permission restrictions don't apply)
        if (process.platform === 'win32' || process.getuid?.() === 0) {
          return;
        }

        await fs.chmod(tempFile, 0o000);
        const result = await validatePath(tempFile, {
          readable: true,
        });
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('not readable');
          expect(result.guidance?.hint).toContain('cannot be read');
          expect(result.guidance?.resolution).toContain('read access');
        }
        // Restore permissions for cleanup
        await fs.chmod(tempFile, 0o644);
      });
    });

    describe('writable validation', () => {
      it('should validate that an existing path is writable', async () => {
        const result = await validatePath(tempFile, {
          writable: true,
        });
        expect(result.ok).toBe(true);
      });

      it('should validate that a non-existent path can be created (parent writable)', async () => {
        const newFile = path.join(tempDir, 'new-file.txt');
        const result = await validatePath(newFile, {
          writable: true,
        });
        expect(result.ok).toBe(true);
      });

      it('should fail when path is not writable', async () => {
        // Skip this test when running as root (permission restrictions don't apply)
        if (process.platform === 'win32' || process.getuid?.() === 0) {
          return;
        }

        await fs.chmod(tempFile, 0o444);
        const result = await validatePath(tempFile, {
          writable: true,
        });
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('not writable');
          expect(result.guidance?.hint).toContain('cannot be written');
          expect(result.guidance?.resolution).toContain('write access');
        }
        // Restore permissions for cleanup
        await fs.chmod(tempFile, 0o644);
      });

      it('should fail when parent directory is not writable for non-existent path', async () => {
        // Skip this test when running as root (permission restrictions don't apply)
        if (process.platform === 'win32' || process.getuid?.() === 0) {
          return;
        }

        await fs.chmod(tempDir, 0o555);
        const newFile = path.join(tempDir, 'new-file.txt');
        const result = await validatePath(newFile, {
          writable: true,
        });
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('not writable');
          expect(result.guidance?.hint).toContain('not writable');
        }
        // Restore permissions for cleanup
        await fs.chmod(tempDir, 0o755);
      });
    });

    describe('combined validation options', () => {
      it('should validate readable and writable together', async () => {
        const result = await validatePath(tempFile, {
          readable: true,
          writable: true,
        });
        expect(result.ok).toBe(true);
      });

      it('should validate existence, type, and permissions together', async () => {
        const result = await validatePath(tempFile, {
          mustExist: true,
          mustBeFile: true,
          readable: true,
          writable: true,
        });
        expect(result.ok).toBe(true);
      });

      it('should fail validation when any option fails', async () => {
        // Skip this test when running as root (permission restrictions don't apply)
        if (process.platform === 'win32' || process.getuid?.() === 0) {
          return;
        }

        await fs.chmod(tempFile, 0o444);
        const result = await validatePath(tempFile, {
          mustExist: true,
          mustBeFile: true,
          readable: true,
          writable: true,
        });
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('not writable');
        }
        // Restore permissions for cleanup
        await fs.chmod(tempFile, 0o644);
      });
    });

    describe('error guidance', () => {
      it('should provide helpful guidance for nonexistent paths', async () => {
        const result = await validatePath('/nonexistent/path', { mustExist: true });
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.guidance?.hint).toContain('could not be found');
          expect(result.guidance?.resolution).toContain('Verify the path');
        }
      });
    });
  });

  describe('validateImageName', () => {
    describe('valid image names', () => {
      it('should validate simple image names', () => {
        const result = validateImageName('nginx');
        expect(result.ok).toBe(true);
      });

      it('should validate image names with tags', () => {
        const result = validateImageName('nginx:latest');
        expect(result.ok).toBe(true);
      });

      it('should validate image names with registry', () => {
        const result = validateImageName('docker.io/library/nginx:latest');
        expect(result.ok).toBe(true);
      });

      it('should validate image names with namespace', () => {
        const result = validateImageName('myorg/myapp:v1.0.0');
        expect(result.ok).toBe(true);
      });

      it('should validate image names with registry and namespace', () => {
        const result = validateImageName('gcr.io/my-project/my-app:1.0.0');
        expect(result.ok).toBe(true);
      });

      it('should validate image names with port in registry', () => {
        const result = validateImageName('localhost:5000/myapp:latest');
        expect(result.ok).toBe(true);
      });

      it('should validate image names with hyphens and underscores', () => {
        expect(validateImageName('my-app:latest').ok).toBe(true);
        expect(validateImageName('my_app:latest').ok).toBe(true);
        expect(validateImageName('my.app:latest').ok).toBe(true);
      });

      it('should validate complex image names', () => {
        const result = validateImageName('registry.example.com:443/my-org/my-app:v1.0.0-alpha.1');
        expect(result.ok).toBe(true);
      });
    });

    describe('invalid image names', () => {
      it('should reject empty image names', () => {
        const result = validateImageName('');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('cannot be empty');
          expect(result.guidance?.resolution).toContain('Provide a valid image name');
        }
      });

      it('should reject image names with invalid characters', () => {
        const result = validateImageName('my app:latest');
        expect(result.ok).toBe(false);
      });

      it('should reject image names starting with special characters', () => {
        expect(validateImageName('-myapp:latest').ok).toBe(false);
        expect(validateImageName('.myapp:latest').ok).toBe(false);
      });

      it('should reject image names that are too long', () => {
        const longName = 'a'.repeat(256) + ':latest';
        const result = validateImageName(longName);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('too long');
          expect(result.guidance?.details?.length).toBe(longName.length);
        }
      });

      it('should provide helpful guidance for invalid names', () => {
        const result = validateImageName('INVALID NAME');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.guidance?.hint).toContain('pattern');
          expect(result.guidance?.resolution).toContain('lowercase');
        }
      });
    });
  });

  describe('validateK8sName', () => {
    describe('valid Kubernetes names', () => {
      it('should validate simple lowercase names', () => {
        const result = validateK8sName('myapp');
        expect(result.ok).toBe(true);
      });

      it('should validate names with hyphens', () => {
        const result = validateK8sName('my-app');
        expect(result.ok).toBe(true);
      });

      it('should validate names with numbers', () => {
        const result = validateK8sName('app-123');
        expect(result.ok).toBe(true);
      });

      it('should validate names starting with numbers', () => {
        const result = validateK8sName('1app');
        expect(result.ok).toBe(true);
      });

      it('should validate single character names', () => {
        expect(validateK8sName('a').ok).toBe(true);
        expect(validateK8sName('1').ok).toBe(true);
      });

      it('should validate complex valid names', () => {
        expect(validateK8sName('my-app-deployment-v1').ok).toBe(true);
        expect(validateK8sName('frontend-service-8080').ok).toBe(true);
      });
    });

    describe('invalid Kubernetes names', () => {
      it('should reject empty names', () => {
        const result = validateK8sName('');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('cannot be empty');
        }
      });

      it('should reject uppercase names', () => {
        const result = validateK8sName('MyApp');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('lowercase');
        }
      });

      it('should reject names with underscores', () => {
        const result = validateK8sName('my_app');
        expect(result.ok).toBe(false);
      });

      it('should reject names starting with hyphens', () => {
        const result = validateK8sName('-myapp');
        expect(result.ok).toBe(false);
      });

      it('should reject names ending with hyphens', () => {
        const result = validateK8sName('myapp-');
        expect(result.ok).toBe(false);
      });

      it('should reject names that are too long', () => {
        const longName = 'a'.repeat(254);
        const result = validateK8sName(longName);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('too long');
          expect(result.guidance?.details?.maxLength).toBe(253);
        }
      });

      it('should reject names with special characters', () => {
        expect(validateK8sName('my.app').ok).toBe(false);
        expect(validateK8sName('my@app').ok).toBe(false);
        expect(validateK8sName('my app').ok).toBe(false);
      });

      it('should provide helpful guidance for invalid names', () => {
        const result = validateK8sName('My_App');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.guidance?.resolution).toContain('lowercase');
          expect(result.guidance?.resolution).toContain('hyphens');
        }
      });
    });
  });

  describe('validateEnvName', () => {
    describe('valid environment variable names', () => {
      it('should validate uppercase names', () => {
        const result = validateEnvName('MYVAR');
        expect(result.ok).toBe(true);
      });

      it('should validate names with underscores', () => {
        const result = validateEnvName('MY_VAR');
        expect(result.ok).toBe(true);
      });

      it('should validate names with numbers', () => {
        const result = validateEnvName('VAR_123');
        expect(result.ok).toBe(true);
      });

      it('should validate names starting with underscores', () => {
        const result = validateEnvName('_PRIVATE');
        expect(result.ok).toBe(true);
      });

      it('should validate complex valid names', () => {
        expect(validateEnvName('DATABASE_CONNECTION_STRING').ok).toBe(true);
        expect(validateEnvName('API_KEY_V2').ok).toBe(true);
        expect(validateEnvName('_INTERNAL_VAR_123').ok).toBe(true);
      });
    });

    describe('invalid environment variable names', () => {
      it('should reject empty names', () => {
        const result = validateEnvName('');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('cannot be empty');
        }
      });

      it('should reject lowercase names', () => {
        const result = validateEnvName('myvar');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('uppercase');
        }
      });

      it('should reject mixed case names', () => {
        const result = validateEnvName('MyVar');
        expect(result.ok).toBe(false);
      });

      it('should reject names with hyphens', () => {
        const result = validateEnvName('MY-VAR');
        expect(result.ok).toBe(false);
      });

      it('should reject names starting with numbers', () => {
        const result = validateEnvName('123_VAR');
        expect(result.ok).toBe(false);
      });

      it('should reject names with special characters', () => {
        expect(validateEnvName('MY.VAR').ok).toBe(false);
        expect(validateEnvName('MY@VAR').ok).toBe(false);
        expect(validateEnvName('MY VAR').ok).toBe(false);
      });

      it('should provide helpful guidance for invalid names', () => {
        const result = validateEnvName('my-var');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.guidance?.resolution).toContain('uppercase');
          expect(result.guidance?.resolution).toContain('underscores');
        }
      });
    });
  });

  describe('validatePort', () => {
    describe('valid ports', () => {
      it('should validate common ports', () => {
        expect(validatePort(80).ok).toBe(true);
        expect(validatePort(443).ok).toBe(true);
        expect(validatePort(8080).ok).toBe(true);
        expect(validatePort(3000).ok).toBe(true);
      });

      it('should validate minimum port (1)', () => {
        const result = validatePort(1);
        expect(result.ok).toBe(true);
      });

      it('should validate maximum port (65535)', () => {
        const result = validatePort(65535);
        expect(result.ok).toBe(true);
      });

      it('should validate all valid port numbers', () => {
        expect(validatePort(1).ok).toBe(true);
        expect(validatePort(1024).ok).toBe(true);
        expect(validatePort(49152).ok).toBe(true);
        expect(validatePort(65535).ok).toBe(true);
      });
    });

    describe('invalid ports', () => {
      it('should reject port 0', () => {
        const result = validatePort(0);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('between 1 and 65535');
        }
      });

      it('should reject negative ports', () => {
        const result = validatePort(-1);
        expect(result.ok).toBe(false);
      });

      it('should reject ports above 65535', () => {
        const result = validatePort(65536);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('between 1 and 65535');
        }
      });

      it('should reject non-integer ports', () => {
        expect(validatePort(80.5).ok).toBe(false);
        expect(validatePort(3000.1).ok).toBe(false);
      });

      it('should provide helpful guidance for invalid ports', () => {
        const result = validatePort(70000);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.guidance?.hint).toContain('Valid port numbers');
          expect(result.guidance?.details?.validRange).toBe('1-65535');
        }
      });
    });
  });

  describe('validateNamespace', () => {
    describe('valid namespaces', () => {
      it('should validate simple lowercase names', () => {
        const result = validateNamespace('production');
        expect(result.ok).toBe(true);
      });

      it('should validate namespaces with hyphens', () => {
        const result = validateNamespace('my-namespace');
        expect(result.ok).toBe(true);
      });

      it('should validate namespaces with numbers', () => {
        const result = validateNamespace('prod-123');
        expect(result.ok).toBe(true);
      });

      it('should validate common namespace names', () => {
        expect(validateNamespace('default').ok).toBe(true);
        expect(validateNamespace('kube-system').ok).toBe(true);
        expect(validateNamespace('kube-public').ok).toBe(true);
      });
    });

    describe('invalid namespaces', () => {
      it('should reject empty namespaces', () => {
        const result = validateNamespace('');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('cannot be empty');
        }
      });

      it('should reject uppercase namespaces', () => {
        const result = validateNamespace('Production');
        expect(result.ok).toBe(false);
      });

      it('should reject namespaces with underscores', () => {
        const result = validateNamespace('my_namespace');
        expect(result.ok).toBe(false);
      });

      it('should reject namespaces that are too long', () => {
        const longName = 'a'.repeat(64);
        const result = validateNamespace(longName);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('too long');
          expect(result.guidance?.details?.maxLength).toBe(63);
        }
      });

      it('should reject namespaces starting with hyphens', () => {
        const result = validateNamespace('-namespace');
        expect(result.ok).toBe(false);
      });

      it('should reject namespaces ending with hyphens', () => {
        const result = validateNamespace('namespace-');
        expect(result.ok).toBe(false);
      });

      it('should provide helpful guidance for invalid namespaces', () => {
        const result = validateNamespace('My_Namespace');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.guidance?.resolution).toContain('lowercase');
        }
      });
    });
  });

  describe('validateDockerTag', () => {
    describe('valid Docker tags', () => {
      it('should validate simple tags', () => {
        const result = validateDockerTag('latest');
        expect(result.ok).toBe(true);
      });

      it('should validate semantic version tags', () => {
        expect(validateDockerTag('v1.0.0').ok).toBe(true);
        expect(validateDockerTag('1.0.0').ok).toBe(true);
        expect(validateDockerTag('2.1.3').ok).toBe(true);
      });

      it('should validate tags with hyphens', () => {
        const result = validateDockerTag('1.0.0-alpha');
        expect(result.ok).toBe(true);
      });

      it('should validate tags with underscores', () => {
        const result = validateDockerTag('my_tag');
        expect(result.ok).toBe(true);
      });

      it('should validate tags with periods', () => {
        const result = validateDockerTag('v1.0.0-alpha.1');
        expect(result.ok).toBe(true);
      });

      it('should validate complex tags', () => {
        expect(validateDockerTag('1.0.0-rc.1').ok).toBe(true);
        expect(validateDockerTag('v2.0.0-beta.2').ok).toBe(true);
        expect(validateDockerTag('20231015-abc123').ok).toBe(true);
      });

      it('should validate tags with mixed case', () => {
        expect(validateDockerTag('Latest').ok).toBe(true);
        expect(validateDockerTag('V1.0.0').ok).toBe(true);
      });
    });

    describe('invalid Docker tags', () => {
      it('should reject empty tags', () => {
        const result = validateDockerTag('');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('cannot be empty');
        }
      });

      it('should reject tags starting with period', () => {
        const result = validateDockerTag('.tag');
        expect(result.ok).toBe(false);
      });

      it('should reject tags starting with hyphen', () => {
        const result = validateDockerTag('-tag');
        expect(result.ok).toBe(false);
      });

      it('should reject tags that are too long', () => {
        const longTag = 'a'.repeat(129);
        const result = validateDockerTag(longTag);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('too long');
          expect(result.guidance?.details?.maxLength).toBe(128);
        }
      });

      it('should reject tags with spaces', () => {
        const result = validateDockerTag('my tag');
        expect(result.ok).toBe(false);
      });

      it('should reject tags with special characters', () => {
        expect(validateDockerTag('tag@version').ok).toBe(false);
        expect(validateDockerTag('tag#1').ok).toBe(false);
        expect(validateDockerTag('tag$version').ok).toBe(false);
      });

      it('should provide helpful guidance for invalid tags', () => {
        const result = validateDockerTag('.invalid-tag');
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.guidance?.hint).toBeDefined();
          expect(result.guidance?.resolution).toContain('Cannot start with period');
        }
      });
    });
  });

  describe('error guidance consistency', () => {
    it('should provide consistent error structure across all validators', async () => {
      const validators = [
        validateImageName(''),
        validateK8sName(''),
        validateEnvName(''),
        validatePort(-1),
        validateNamespace(''),
        validateDockerTag(''),
      ];

      for (const result of validators) {
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toBeDefined();
          expect(typeof result.error).toBe('string');
          expect(result.guidance?.message).toBeDefined();
          expect(result.guidance?.hint).toBeDefined();
          expect(result.guidance?.resolution).toBeDefined();
        }
      }
    });

    it('should include actionable resolution steps', () => {
      const invalidResults = [
        validateImageName('INVALID NAME'),
        validateK8sName('My_App'),
        validateEnvName('my-var'),
        validatePort(70000),
        validateNamespace('My_Namespace'),
        validateDockerTag('.tag'),
      ];

      for (const result of invalidResults) {
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.guidance?.resolution).toMatch(/[A-Z]/); // Should start with capital
          expect(result.guidance?.resolution.length).toBeGreaterThan(10); // Should be meaningful
        }
      }
    });
  });
});
