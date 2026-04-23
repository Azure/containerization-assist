/**
 * Helm Knowledge Pack Tests
 *
 * Validates the helm-pack.json knowledge pack:
 * - Schema conformance (every entry passes KnowledgeEntrySchema)
 * - Structural invariants (IDs unique, required tags present, correct category)
 * - Runtime matching (entries surface for realistic Helm-related queries)
 */

import { describe, it, expect, beforeAll } from '@jest/globals';
import { readFileSync } from 'fs';
import { join } from 'path';
import { KnowledgePackSchema } from '@/knowledge/schemas';
import { findKnowledgeMatches } from '@/knowledge/matcher';
import { loadKnowledgeBase, getAllEntries } from '@/knowledge/loader';
import type { KnowledgeEntry, KnowledgeQuery, LoadedEntry } from '@/knowledge/types';

const PACK_PATH = join(process.cwd(), 'knowledge/packs/helm-pack.json');

function loadPack(): KnowledgeEntry[] {
  return JSON.parse(readFileSync(PACK_PATH, 'utf-8')) as KnowledgeEntry[];
}

describe('Helm Knowledge Pack', () => {
  describe('schema validation', () => {
    it('validates against KnowledgePackSchema', () => {
      const raw = readFileSync(PACK_PATH, 'utf-8');
      const data: unknown = JSON.parse(raw);
      const result = KnowledgePackSchema.safeParse(data);

      if (!result.success) {
        // Surface the first error for easier debugging
        const firstIssue = result.error.issues[0];
        throw new Error(
          `Schema validation failed at path ${firstIssue?.path.join('.')}: ${firstIssue?.message}`,
        );
      }
      expect(result.success).toBe(true);
    });
  });

  describe('structural invariants', () => {
    let entries: KnowledgeEntry[];

    beforeAll(() => {
      entries = loadPack();
    });

    it('has a reasonable number of entries', () => {
      expect(entries.length).toBeGreaterThanOrEqual(40);
    });

    it('has unique IDs across all entries', () => {
      const ids = entries.map((e) => e.id);
      const uniqueIds = new Set(ids);
      const duplicates = ids.filter((id, i) => ids.indexOf(id) !== i);
      expect(duplicates).toEqual([]);
      expect(uniqueIds.size).toBe(entries.length);
    });

    it('all entries use category "kubernetes"', () => {
      for (const entry of entries) {
        expect(entry.category).toBe('kubernetes');
      }
    });

    it('all entries include "helm" tag', () => {
      for (const entry of entries) {
        expect(entry.tags).toBeDefined();
        expect(entry.tags).toContain('helm');
      }
    });

    it('all entries include "generate-k8s-manifests" tag', () => {
      for (const entry of entries) {
        expect(entry.tags).toBeDefined();
        expect(entry.tags).toContain('generate-k8s-manifests');
      }
    });

    it('all entries have non-empty patterns', () => {
      for (const entry of entries) {
        expect(entry.pattern.length).toBeGreaterThan(0);
      }
    });

    it('all entry patterns compile as valid regular expressions', () => {
      for (const entry of entries) {
        expect(() => {
          try {
            new RegExp(entry.pattern, 'gmi');
          } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            throw new Error(
              `Invalid regex pattern for entry "${entry.id}" (${entry.pattern}): ${message}`,
            );
          }
        }).not.toThrow();
      }
    });

    it('all entries have non-empty recommendations', () => {
      for (const entry of entries) {
        expect(entry.recommendation.length).toBeGreaterThan(0);
      }
    });

    it('all entries have a severity level', () => {
      const validSeverities = ['required', 'high', 'medium', 'low'];
      for (const entry of entries) {
        expect(entry.severity).toBeDefined();
        expect(validSeverities).toContain(entry.severity);
      }
    });

    it('covers expected topic areas', () => {
      const allText = JSON.stringify(entries).toLowerCase();
      const expectedTopics = [
        'chart.yaml',
        'values.yaml',
        '_helpers.tpl',
        'templates',
        'subchart',
        'rbac',
        'security',
        'resource',
      ];
      for (const topic of expectedTopics) {
        expect(allText).toContain(topic);
      }
    });
  });

  describe('matcher integration', () => {
    let helmEntries: LoadedEntry[];

    beforeAll(() => {
      loadKnowledgeBase();
      helmEntries = getAllEntries().filter((e) => e.tags?.includes('helm'));
    });

    it('helm entries are loaded into the knowledge base', () => {
      expect(helmEntries.length).toBeGreaterThanOrEqual(40);
    });

    it('matches Helm chart structure queries', () => {
      const query: KnowledgeQuery = {
        category: 'kubernetes',
        text: 'Chart.yaml apiVersion name version',
        tags: ['helm'],
      };
      const matches = findKnowledgeMatches(helmEntries, query);
      expect(matches.length).toBeGreaterThan(0);
      expect(matches[0].score).toBeGreaterThan(0);
    });

    it('matches Helm template helper queries', () => {
      const query: KnowledgeQuery = {
        category: 'kubernetes',
        text: 'define helpers include nindent fullname trunc',
        tags: ['helm'],
      };
      const matches = findKnowledgeMatches(helmEntries, query);
      expect(matches.length).toBeGreaterThan(0);

      const matchIds = matches.map((m) => m.entry.id);
      expect(matchIds).toContain('helm-fullname-63-char-trunc');
    });

    it('matches Helm security queries', () => {
      const query: KnowledgeQuery = {
        category: 'kubernetes',
        text: 'securityContext runAsNonRoot readOnlyRootFilesystem',
        tags: ['helm'],
      };
      const matches = findKnowledgeMatches(helmEntries, query);
      expect(matches.length).toBeGreaterThan(0);
    });

    it('matches Helm values best practice queries', () => {
      const query: KnowledgeQuery = {
        category: 'kubernetes',
        text: 'values.yaml camelCase defaults',
        tags: ['helm'],
      };
      const matches = findKnowledgeMatches(helmEntries, query);
      expect(matches.length).toBeGreaterThan(0);
    });

    it('does not match non-helm categories', () => {
      const query: KnowledgeQuery = {
        category: 'dockerfile',
        text: 'helm chart',
      };
      const matches = findKnowledgeMatches(helmEntries, query);
      // All helm entries are category=kubernetes, so dockerfile filter should exclude them
      expect(matches).toHaveLength(0);
    });
  });
});
