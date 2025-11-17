/**
 * Unit tests for Policy IO (Unified Rego + CEL Loading)
 */

import { loadPolicy, loadAndMergePolicies, clearPolicyCache } from '@/config/policy-io';
import { writeFile, unlink, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import os from 'node:os';
import { existsSync } from 'node:fs';

describe('Policy IO', () => {
  let tmpDir: string;

  beforeAll(async () => {
    tmpDir = join(os.tmpdir(), `policy-io-test-${Date.now()}`);
    await mkdir(tmpDir, { recursive: true });
  });

  beforeEach(() => {
    // Clear cache before each test
    clearPolicyCache();
  });

  afterEach(async () => {
    // Clean up temp files
    try {
      const files = ['test.rego', 'test.yaml', 'test.yml', 'test.txt'];
      for (const file of files) {
        const filePath = join(tmpDir, file);
        if (existsSync(filePath)) {
          await unlink(filePath);
        }
      }
    } catch {
      // Ignore cleanup errors
    }
  });

  describe('loadPolicy', () => {
    describe('Rego policies', () => {
      const validRegoContent = `
package containerization.test

violations contains result if {
  not regex.match("USER", input.content)
  result := {
    "rule": "require-user",
    "category": "security",
    "severity": "block",
    "message": "USER directive required"
  }
}
`;

      it('should load valid .rego policy', async () => {
        const regoFile = join(tmpDir, 'test.rego');
        await writeFile(regoFile, validRegoContent);

        const result = await loadPolicy(regoFile);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.policyPaths).toContain(regoFile);
        }
      });

      it('should cache loaded Rego policies', async () => {
        const regoFile = join(tmpDir, 'test.rego');
        await writeFile(regoFile, validRegoContent);

        const result1 = await loadPolicy(regoFile);
        const result2 = await loadPolicy(regoFile);

        expect(result1.ok).toBe(true);
        expect(result2.ok).toBe(true);
        // Should be same cached instance
        if (result1.ok && result2.ok) {
          expect(result1.value).toBe(result2.value);
        }
      });
    });

    describe('CEL policies', () => {
      const validCelContent = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: test-policy
spec:
  rules:
    - name: require-user
      category: security
      severity: warn
      condition: '!input.content.contains("USER")'
      message: "Dockerfile should specify USER"
`;

      it('should load valid .yaml CEL policy', async () => {
        const yamlFile = join(tmpDir, 'test.yaml');
        await writeFile(yamlFile, validCelContent);

        const result = await loadPolicy(yamlFile);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.policyPaths).toContain(yamlFile);
        }
      });

      it('should load valid .yml CEL policy', async () => {
        const ymlFile = join(tmpDir, 'test.yml');
        await writeFile(ymlFile, validCelContent);

        const result = await loadPolicy(ymlFile);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.policyPaths).toContain(ymlFile);
        }
      });

      it('should cache loaded CEL policies', async () => {
        const yamlFile = join(tmpDir, 'test.yaml');
        await writeFile(yamlFile, validCelContent);

        const result1 = await loadPolicy(yamlFile);
        const result2 = await loadPolicy(yamlFile);

        expect(result1.ok).toBe(true);
        expect(result2.ok).toBe(true);
        // Should be same cached instance
        if (result1.ok && result2.ok) {
          expect(result1.value).toBe(result2.value);
        }
      });
    });

    describe('Error cases', () => {
      it('should reject unsupported file extension', async () => {
        const txtFile = join(tmpDir, 'test.txt');
        await writeFile(txtFile, 'some content');

        const result = await loadPolicy(txtFile);

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Unsupported policy file format');
        }
      });

      it('should reject non-existent file', async () => {
        const result = await loadPolicy('/nonexistent/policy.rego');

        expect(result.ok).toBe(false);
      });

      it('should handle invalid CEL YAML', async () => {
        const yamlFile = join(tmpDir, 'test.yaml');
        await writeFile(
          yamlFile,
          `
apiVersion: wrong-version
kind: PolicySet
metadata:
  name: test
spec:
  rules: []
`
        );

        const result = await loadPolicy(yamlFile);

        expect(result.ok).toBe(false);
      });
    });
  });

  describe('loadAndMergePolicies', () => {
    const validRegoContent = `
package containerization.test

violations contains result if {
  not regex.match("USER", input.content)
  result := {
    "rule": "require-user-rego",
    "category": "security",
    "severity": "block",
    "message": "USER directive required (Rego)"
  }
}
`;

    const validCelContent = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: cel-policy
spec:
  rules:
    - name: require-healthcheck-cel
      category: health
      severity: warn
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "HEALTHCHECK recommended (CEL)"
`;

    it('should load and merge multiple Rego policies', async () => {
      const rego1 = join(tmpDir, 'rego1.rego');
      const rego2 = join(tmpDir, 'rego2.rego');

      await writeFile(rego1, validRegoContent);
      await writeFile(rego2, validRegoContent);

      const result = await loadAndMergePolicies([rego1, rego2]);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.policyPaths.length).toBeGreaterThan(0);
      }

      await unlink(rego1);
      await unlink(rego2);
    });

    it('should load and merge multiple CEL policies', async () => {
      const cel1 = join(tmpDir, 'cel1.yaml');
      const cel2 = join(tmpDir, 'cel2.yml');

      await writeFile(cel1, validCelContent);
      await writeFile(cel2, validCelContent);

      const result = await loadAndMergePolicies([cel1, cel2]);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.policyPaths).toHaveLength(2);
      }

      await unlink(cel1);
      await unlink(cel2);
    });

    it('should load and merge mixed Rego + CEL policies', async () => {
      const regoFile = join(tmpDir, 'mixed.rego');
      const celFile = join(tmpDir, 'mixed.yaml');

      await writeFile(regoFile, validRegoContent);
      await writeFile(celFile, validCelContent);

      const result = await loadAndMergePolicies([regoFile, celFile]);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.policyPaths).toHaveLength(2);

        // Test that it actually evaluates both policies
        const evalResult = await result.value.evaluate('FROM alpine\nRUN echo hello');
        // Should have violations/warnings from both policies
        const totalIssues =
          evalResult.violations.length + evalResult.warnings.length + evalResult.suggestions.length;
        expect(totalIssues).toBeGreaterThan(0);
      }

      await unlink(regoFile);
      await unlink(celFile);
    });

    it('should return single policy directly when only one provided', async () => {
      const celFile = join(tmpDir, 'single.yaml');
      await writeFile(celFile, validCelContent);

      const result = await loadAndMergePolicies([celFile]);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.policyPaths).toHaveLength(1);
      }

      await unlink(celFile);
    });

    it('should reject empty policy paths array', async () => {
      const result = await loadAndMergePolicies([]);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No policy paths provided');
      }
    });

    it('should reject invalid file extensions', async () => {
      const txtFile = join(tmpDir, 'invalid.txt');
      await writeFile(txtFile, 'content');

      const result = await loadAndMergePolicies([txtFile]);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid policy file formats');
      }

      await unlink(txtFile);
    });

    it('should fail if any policy fails to load', async () => {
      const validFile = join(tmpDir, 'valid.yaml');
      await writeFile(validFile, validCelContent);

      const result = await loadAndMergePolicies([
        validFile,
        '/nonexistent/policy.rego',
      ]);

      expect(result.ok).toBe(false);

      await unlink(validFile);
    });
  });

  describe('clearPolicyCache', () => {
    it('should clear cached policies', async () => {
      const celContent = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: cache-test
spec:
  rules:
    - name: test
      category: test
      severity: suggest
      condition: 'true'
      message: "Test"
`;

      const yamlFile = join(tmpDir, 'cache-test.yaml');
      await writeFile(yamlFile, celContent);

      // Load and cache
      const result1 = await loadPolicy(yamlFile);
      expect(result1.ok).toBe(true);

      // Clear cache
      clearPolicyCache();

      // Load again - should be different instance
      const result2 = await loadPolicy(yamlFile);
      expect(result2.ok).toBe(true);

      // Should be different instances (not cached)
      if (result1.ok && result2.ok) {
        expect(result1.value).not.toBe(result2.value);
      }

      await unlink(yamlFile);
    });
  });
});
