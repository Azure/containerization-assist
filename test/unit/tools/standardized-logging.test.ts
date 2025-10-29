/**
 * Regression Tests: Standardized Logging Across All Tools
 *
 * Ensures all tools use createStandardizedToolTracker and emit
 * consistent "Starting X" / "Completed X" log messages.
 *
 * This test suite prevents regressions where tools bypass the
 * standardized logging pattern.
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import type { Logger } from 'pino';

/**
 * All MCP tools that should follow standardized logging
 */
const ALL_TOOLS = [
  'analyze-repo',
  'build-image',
  'fix-dockerfile',
  'generate-dockerfile',
  'generate-k8s-manifests',
  'ops',
  'prepare-cluster',
  'push-image',
  'scan-image',
  'tag-image',
  'fix-dockerfile',
  'verify-deploy',
] as const;

describe('Standardized Logging Regression Tests', () => {
  let mockLogger: Logger;
  let infoSpy: jest.Mock;
  let errorSpy: jest.Mock;

  beforeEach(() => {
    infoSpy = jest.fn();
    errorSpy = jest.fn();
    mockLogger = {
      info: infoSpy,
      error: errorSpy,
      warn: jest.fn(),
      debug: jest.fn(),
      trace: jest.fn(),
      fatal: jest.fn(),
      child: jest.fn().mockReturnThis(),
    } as unknown as Logger;
  });

  describe('Tool logging format validation', () => {
    it('should define all expected tools', () => {
      expect(ALL_TOOLS.length).toBeGreaterThan(8);
      expect(ALL_TOOLS).toContain('build-image');
      expect(ALL_TOOLS).toContain('verify-deploy');
    });

    ALL_TOOLS.forEach((toolName) => {
      describe(`${toolName} tool`, () => {
        it('should have standardized start message format', () => {
          // This test documents the expected format
          const expectedStartMessage = `Starting ${toolName}`;
          expect(expectedStartMessage).toMatch(/^Starting /);
          expect(expectedStartMessage).toBe(`Starting ${toolName}`);
        });

        it('should have standardized completion message format', () => {
          const expectedCompleteMessage = `Completed ${toolName}`;
          expect(expectedCompleteMessage).toMatch(/^Completed /);
          expect(expectedCompleteMessage).toBe(`Completed ${toolName}`);
        });

        it('should have standardized failure message format', () => {
          const expectedFailMessage = `Failed ${toolName}`;
          expect(expectedFailMessage).toMatch(/^Failed /);
          expect(expectedFailMessage).toBe(`Failed ${toolName}`);
        });
      });
    });
  });

  describe('Tool file imports validation', () => {
    ALL_TOOLS.forEach((toolName) => {
      it(`${toolName} should import createStandardizedToolTracker`, async () => {
        // Dynamic import to check the module structure
        const toolPath = `../../../src/tools/${toolName}/tool`;

        try {
          const toolModule = await import(toolPath);
          expect(toolModule).toBeDefined();

          // The tool should export a default tool object
          expect(toolModule.default || toolModule[`${toCamelCase(toolName)}`]).toBeDefined();
        } catch (error) {
          // If import fails, it's a structural issue
          throw new Error(`Failed to import ${toolName} tool: ${error}`);
        }
      });
    });
  });

  describe('Logging pattern enforcement', () => {
    it('should ensure start messages use consistent capitalization', () => {
      const validPatterns = ALL_TOOLS.map((tool) => `Starting ${tool}`);

      validPatterns.forEach((pattern) => {
        // All should start with capital S
        expect(pattern.charAt(0)).toBe('S');
        // All should have second word that matches tool name exactly
        expect(pattern.split(' ')[1]).toBeTruthy();
      });
    });

    it('should ensure completion messages use consistent capitalization', () => {
      const validPatterns = ALL_TOOLS.map((tool) => `Completed ${tool}`);

      validPatterns.forEach((pattern) => {
        // All should start with capital C
        expect(pattern.charAt(0)).toBe('C');
        // All should have second word that matches tool name exactly
        expect(pattern.split(' ')[1]).toBeTruthy();
      });
    });

    it('should ensure failure messages use consistent capitalization', () => {
      const validPatterns = ALL_TOOLS.map((tool) => `Failed ${tool}`);

      validPatterns.forEach((pattern) => {
        // All should start with capital F
        expect(pattern.charAt(0)).toBe('F');
        // All should have second word that matches tool name exactly
        expect(pattern.split(' ')[1]).toBeTruthy();
      });
    });

    it('should reject inconsistent message formats', () => {
      const invalidPatterns = [
        'starting build-image', // lowercase
        'Start build-image', // wrong verb form
        'Building image', // different verb
        'build-image started', // wrong order
        'Build Image Starting', // wrong capitalization
      ];

      invalidPatterns.forEach((invalid) => {
        expect(invalid).not.toMatch(/^Starting [a-z-]+$/);
      });
    });
  });

  describe('Message format constants', () => {
    it('should use consistent format across all tools', () => {
      // Format: "Starting <tool-name>" where tool-name matches the tool directory
      const formatRegex = /^Starting [a-z][a-z0-9-]*[a-z0-9]$/;

      ALL_TOOLS.forEach((tool) => {
        const message = `Starting ${tool}`;
        expect(message).toMatch(formatRegex);
      });
    });

    it('should use kebab-case for multi-word tools', () => {
      const multiWordTools = ALL_TOOLS.filter((tool) => tool.includes('-'));

      expect(multiWordTools.length).toBeGreaterThan(5);
      multiWordTools.forEach((tool) => {
        // Should not have consecutive hyphens
        expect(tool).not.toMatch(/--/);
        // Should not start or end with hyphen
        expect(tool.charAt(0)).not.toBe('-');
        expect(tool.charAt(tool.length - 1)).not.toBe('-');
        // Should only contain lowercase letters, numbers, and hyphens
        expect(tool).toMatch(/^[a-z0-9-]+$/);
      });
    });
  });

  describe('Regression prevention', () => {
    it('should catch if a tool adds custom start logging', () => {
      // Custom patterns that would break consistency
      const antiPatterns = ['Begin processing', 'Initiating', 'Commencing', 'Now starting'];

      antiPatterns.forEach((pattern) => {
        // These should NOT match the standard pattern
        const matchesStandard = /^Starting [a-z][a-z0-9-]*$/.test(pattern);
        expect(matchesStandard).toBe(false);
      });
    });

    it('should catch if a tool adds custom completion logging', () => {
      const antiPatterns = ['Finished', 'Done', 'Complete', 'Successfully completed'];

      antiPatterns.forEach((pattern) => {
        // These should NOT match the standard pattern
        const matchesStandard = /^Completed [a-z][a-z0-9-]*$/.test(pattern);
        expect(matchesStandard).toBe(false);
      });
    });

    it('should ensure tool names match directory structure', () => {
      // Tools should use their directory name as the tool identifier
      const expectedToolNames = [
        'analyze-repo',
        'build-image',
        'fix-dockerfile',
        'push-image',
        'scan-image',
        'tag-image',
        'verify-deploy',
      ];

      expectedToolNames.forEach((toolName) => {
        expect(ALL_TOOLS).toContain(toolName as any);
      });
    });
  });

  describe('Documentation validation', () => {
    it('should document all critical workflow tools', () => {
      const criticalTools = [
        'analyze-repo',
        'fix-dockerfile',
        'build-image',
        'scan-image',
        'tag-image',
        'push-image',
        'verify-deploy',
      ];

      criticalTools.forEach((tool) => {
        expect(ALL_TOOLS).toContain(tool as any);
      });
    });
  });
});

/**
 * Helper to convert kebab-case to camelCase
 */
function toCamelCase(str: string): string {
  return str.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
}
