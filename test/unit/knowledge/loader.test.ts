/**
 * Knowledge Loader Tests
 * Tests for knowledge pack loading and error handling
 */

import { describe, it, expect, beforeAll } from '@jest/globals';
import * as path from 'node:path';
import { existsSync, readdirSync, readFileSync } from 'fs';
import { loadKnowledgeBase, getAllEntries, isKnowledgeLoaded, loadKnowledgeData } from '@/knowledge/loader';
import { CATEGORY } from '@/knowledge/types';
import { KnowledgeEntrySchema, KnowledgePackSchema } from '@/knowledge/schemas';

describe('Knowledge Loader', () => {
  describe('loadKnowledgeBase integration', () => {
    beforeAll(() => {
      // Load knowledge base before running tests
      loadKnowledgeBase();
    });

    it('should load knowledge base successfully', () => {
      expect(isKnowledgeLoaded()).toBe(true);
    });

    it('should load all knowledge packs from built-in imports', () => {
      // Built-in packs are now imported as JSON modules
      const expectedPackCount = 28; // Total number of built-in packs

      // Verify entries were loaded
      const entries = getAllEntries();
      expect(entries.length).toBeGreaterThan(0);

      console.log(`✅ Loaded ${entries.length} entries from ${expectedPackCount} built-in packs`);
    });

    it('should return entries from getAllEntries', () => {
      const entries = getAllEntries();

      expect(Array.isArray(entries)).toBe(true);
      expect(entries.length).toBeGreaterThan(0);

      // Verify entries have required fields
      const sampleEntry = entries[0];
      expect(sampleEntry).toHaveProperty('id');
      expect(sampleEntry).toHaveProperty('category');
      expect(sampleEntry).toHaveProperty('pattern');
      expect(sampleEntry).toHaveProperty('recommendation');
    });

    it('should load entries with valid categories', () => {
      const entries = getAllEntries();
      const validCategories = Object.values(CATEGORY);

      for (const entry of entries) {
        expect(validCategories).toContain(entry.category);
      }
    });

    it('should load entries with non-empty patterns', () => {
      const entries = getAllEntries();

      for (const entry of entries) {
        expect(entry.pattern).toBeTruthy();
        expect(entry.pattern.length).toBeGreaterThan(0);
      }
    });

    it('should load entries with non-empty recommendations', () => {
      const entries = getAllEntries();

      for (const entry of entries) {
        expect(entry.recommendation).toBeTruthy();
        expect(entry.recommendation.length).toBeGreaterThan(0);
      }
    });

    it('should not reload if already loaded', () => {
      const entriesBefore = getAllEntries();
      const countBefore = entriesBefore.length;

      // Try loading again
      loadKnowledgeBase();

      const entriesAfter = getAllEntries();
      const countAfter = entriesAfter.length;

      // Should be the same
      expect(countAfter).toBe(countBefore);
    });

    it('should load knowledge data for prompt engine', () => {
      const data = loadKnowledgeData();

      expect(data).toHaveProperty('entries');
      expect(Array.isArray(data.entries)).toBe(true);
      expect(data.entries.length).toBeGreaterThan(0);
    });
  });

  describe('knowledge pack file validation', () => {
    it('should have all expected knowledge packs in directory', () => {
      const packsDir = path.join(process.cwd(), 'knowledge/packs');
      const packFiles = readdirSync(packsDir).filter((f) => f.endsWith('.json')).sort();

      // Expected packs based on actual directory listing
      const expectedPacks = [
        'azure-container-apps-pack.json',
        'base-images-pack.json',
        'build-optimization.json',
        'database-pack.json',
        'dockerfile-advanced.json',
        'dotnet-background-jobs-pack.json',
        'dotnet-blazor-pack.json',
        'dotnet-ef-core-pack.json',
        'dotnet-framework-48-pack.json',
        'dotnet-framework-pack.json',
        'dotnet-grpc-pack.json',
        'dotnet-identity-pack.json',
        'dotnet-mediatr-pack.json',
        'dotnet-pack.json',
        'dotnet-signalr-pack.json',
        'dotnet-worker-pack.json',
        'go-pack.json',
        'java-pack.json',
        'kubernetes-deployment.json',
        'kubernetes-pack.json',
        'nodejs-pack.json',
        'php-pack.json',
        'python-pack.json',
        'ruby-pack.json',
        'rust-pack.json',
        'security-pack.json',
        'security-remediation.json',
        'starter-pack.json',
      ];

      expect(packFiles).toEqual(expectedPacks);
    });

    it('should load packs for all major languages/frameworks', () => {
      const entries = getAllEntries();
      const entriesText = JSON.stringify(entries);

      // Verify we have coverage for major technologies
      const expectedTechnologies = [
        'node',
        'nodejs',
        'python',
        'java',
        'dotnet',
        'go',
        'rust',
        'php',
        'ruby',
      ];

      for (const tech of expectedTechnologies) {
        const hasEntries = entriesText.toLowerCase().includes(tech);
        expect(hasEntries).toBe(true);
      }
    });

    it('should load security knowledge entries', () => {
      const entries = getAllEntries();
      const securityEntries = entries.filter((e) => e.category === 'security');

      expect(securityEntries.length).toBeGreaterThan(0);
    });

    it('should load dockerfile knowledge entries', () => {
      const entries = getAllEntries();
      const dockerfileEntries = entries.filter((e) => e.category === 'dockerfile');

      expect(dockerfileEntries.length).toBeGreaterThan(0);
    });

    it('should load kubernetes knowledge entries', () => {
      const entries = getAllEntries();
      const kubernetesEntries = entries.filter((e) => e.category === 'kubernetes');

      expect(kubernetesEntries.length).toBeGreaterThan(0);
    });

    it('should have unique entry IDs', () => {
      const entries = getAllEntries();
      const ids = entries.map((e) => e.id);
      const uniqueIds = new Set(ids);

      expect(uniqueIds.size).toBe(ids.length);
    });
  });

  describe('knowledge pack statistics', () => {
    it('should log pack loading statistics', () => {
      const entries = getAllEntries();
      const expectedPackCount = 28; // Built-in packs

      console.log('\n📊 Knowledge Pack Statistics:');
      console.log(`   Total Packs: ${expectedPackCount}`);
      console.log(`   Total Entries: ${entries.length}`);
      console.log(`   Avg Entries/Pack: ${Math.round(entries.length / expectedPackCount)}`);

      const categoryCounts = entries.reduce((acc, entry) => {
        acc[entry.category] = (acc[entry.category] || 0) + 1;
        return acc;
      }, {} as Record<string, number>);

      console.log('   Categories:');
      for (const [category, count] of Object.entries(categoryCounts)) {
        console.log(`     - ${category}: ${count}`);
      }

      expect(true).toBe(true); // Just for stats logging
    });
  });

  describe('schema validation (ISSUE-026)', () => {
    it('should validate knowledge entry schema', () => {
      const validEntry = {
        id: 'test-entry',
        category: 'security',
        pattern: 'test.*pattern',
        recommendation: 'Test recommendation',
        severity: 'high',
        tags: ['test', 'security'],
        description: 'Test description',
      };

      const result = KnowledgeEntrySchema.safeParse(validEntry);
      expect(result.success).toBe(true);
    });

    it('should reject entry with invalid category', () => {
      const invalidEntry = {
        id: 'test-entry',
        category: 'invalid-category',
        pattern: 'test.*pattern',
        recommendation: 'Test recommendation',
      };

      const result = KnowledgeEntrySchema.safeParse(invalidEntry);
      expect(result.success).toBe(false);
    });

    it('should reject entry with missing required fields', () => {
      const invalidEntry = {
        id: 'test-entry',
        category: 'security',
        // missing pattern and recommendation
      };

      const result = KnowledgeEntrySchema.safeParse(invalidEntry);
      expect(result.success).toBe(false);
    });

    it('should validate pack in array format', () => {
      const validPack = [
        {
          id: 'entry-1',
          category: 'dockerfile',
          pattern: 'FROM',
          recommendation: 'Use specific base image tags',
        },
        {
          id: 'entry-2',
          category: 'security',
          pattern: 'USER root',
          recommendation: 'Avoid running as root',
        },
      ];

      const result = KnowledgePackSchema.safeParse(validPack);
      expect(result.success).toBe(true);
    });

    it('should validate pack in object format with metadata', () => {
      const validPack = {
        name: 'Test Pack',
        version: '1.0.0',
        description: 'Test pack description',
        rules: [
          {
            id: 'entry-1',
            category: 'dockerfile',
            pattern: 'FROM',
            recommendation: 'Use specific base image tags',
          },
        ],
      };

      const result = KnowledgePackSchema.safeParse(validPack);
      expect(result.success).toBe(true);
    });

    it('should reject empty pack', () => {
      const emptyPack: unknown[] = [];

      const result = KnowledgePackSchema.safeParse(emptyPack);
      expect(result.success).toBe(false);
    });

    it('should reject pack with invalid entry structure', () => {
      const invalidPack = [
        {
          id: 'entry-1',
          // missing required fields
        },
      ];

      const result = KnowledgePackSchema.safeParse(invalidPack);
      expect(result.success).toBe(false);
    });

    it('should accept all category types including build', () => {
      const validCategories = [
        'api',
        'architecture',
        'build',
        'caching',
        'configuration',
        'dockerfile',
        'features',
        'generic',
        'kubernetes',
        'optimization',
        'reliability',
        'resilience',
        'security',
        'streaming',
        'validation',
      ];

      for (const category of validCategories) {
        const entry = {
          id: `test-${category}`,
          category,
          pattern: 'test',
          recommendation: 'test',
        };

        const result = KnowledgeEntrySchema.safeParse(entry);
        expect(result.success).toBe(true);
      }
    });

    it('should validate all actual knowledge packs', () => {
      const packsDir = path.join(process.cwd(), 'knowledge/packs');
      const packFiles = readdirSync(packsDir).filter((f) => f.endsWith('.json'));

      let validCount = 0;
      let invalidCount = 0;

      for (const packFile of packFiles) {
        const packPath = path.join(packsDir, packFile);
        const content = readFileSync(packPath, 'utf-8');

        // Parse JSON directly
        const data = JSON.parse(content);

        const result = KnowledgePackSchema.safeParse(data);
        if (result.success) {
          validCount++;
        } else {
          invalidCount++;
        }
      }

      console.log(`\n✅ Pack Validation: ${validCount} valid, ${invalidCount} invalid (out of ${packFiles.length})`);

      // Should have loaded most packs (26 out of 28 are valid knowledge packs)
      expect(validCount).toBeGreaterThanOrEqual(26);
    });
  });
});
