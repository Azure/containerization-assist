import { findKnowledgeMatches } from '@knowledge/matcher';
import type { LoadedEntry, KnowledgeQuery } from '@knowledge/types';

describe('findKnowledgeMatches', () => {
  let sampleEntries: LoadedEntry[];

  beforeEach(() => {
    
    sampleEntries = [
      {
        id: 'node-alpine',
        category: 'dockerfile',
        pattern: 'node:(?!.*alpine)',
        recommendation: 'Use node:alpine for smaller images',
        severity: 'high',
        tags: ['node', 'alpine', 'optimization']
      },
      {
        id: 'k8s-resources',
        category: 'kubernetes',
        pattern: 'kind:\\s*Deployment',
        recommendation: 'Set resource limits',
        severity: 'high',
        tags: ['resources', 'limits']
      },
      {
        id: 'user-security',
        category: 'security',
        pattern: 'USER\\s+(root|0)',
        recommendation: 'Avoid running as root',
        severity: 'high',
        tags: ['security', 'user']
      }
    ];
  });

  describe('finding matches', () => {
    test('should return matches sorted by score', () => {
      const query: KnowledgeQuery = {
        category: 'dockerfile',
        text: 'FROM node:16'
      };

      const matches = findKnowledgeMatches(sampleEntries, query);
      
      expect(matches.length).toBeGreaterThan(0);
      expect(matches[0].entry.id).toBe('node-alpine');
      expect(matches[0].score).toBeGreaterThan(0);
    });

    test('should filter by category', () => {
      const query: KnowledgeQuery = {
        category: 'kubernetes'
      };

      const matches = findKnowledgeMatches(sampleEntries, query);
      
      expect(matches).toHaveLength(1);
      expect(matches[0].entry.category).toBe('kubernetes');
    });

    test('should match patterns in text', () => {
      const query: KnowledgeQuery = {
        text: 'kind: Deployment'
      };

      const matches = findKnowledgeMatches(sampleEntries, query);
      
      const k8sMatch = matches.find(m => m.entry.id === 'k8s-resources');
      expect(k8sMatch).toBeDefined();
      expect(k8sMatch!.score).toBeGreaterThan(0);
    });

    test('should respect limit parameter', () => {
      const query: KnowledgeQuery = {
        limit: 1
      };

      const matches = findKnowledgeMatches(sampleEntries, query);
      
      expect(matches).toHaveLength(1);
    });

    test('should match by tags', () => {
      const query: KnowledgeQuery = {
        tags: ['security']
      };

      const matches = findKnowledgeMatches(sampleEntries, query);
      
      expect(matches.length).toBeGreaterThan(0);
      expect(matches[0].entry.tags).toContain('security');
    });

    test('should provide reasons for matches', () => {
      const query: KnowledgeQuery = {
        category: 'dockerfile',
        text: 'FROM node:16',
        tags: ['optimization']
      };

      const matches = findKnowledgeMatches(sampleEntries, query);
      
      expect(matches[0].reasons.length).toBeGreaterThan(0);
      expect(matches[0].reasons.some(r => r.includes('Category'))).toBe(true);
    });

    test('should handle language context', () => {
      const query: KnowledgeQuery = {
        language: 'javascript',
        text: 'FROM node:16'
      };

      const matches = findKnowledgeMatches(sampleEntries, query);
      
      const nodeMatch = matches.find(m => m.entry.tags?.includes('node'));
      expect(nodeMatch?.score).toBeGreaterThan(0);
    });

    test('should handle empty query gracefully', () => {
      const query: KnowledgeQuery = {};

      const matches = findKnowledgeMatches(sampleEntries, query);
      
      // Should return all entries sorted by severity/score
      expect(matches.length).toBe(sampleEntries.length);
    });

    test('should handle invalid patterns gracefully', () => {
      const entriesWithInvalidPattern: KnowledgeEntry[] = [
        {
          id: 'invalid-pattern',
          category: 'dockerfile',
          pattern: '[unclosed',  // Invalid regex
          recommendation: 'Test recommendation'
        }
      ];

      const query: KnowledgeQuery = {
        text: 'some text'
      };

      // Should not throw error
      expect(() => {
        findKnowledgeMatches(entriesWithInvalidPattern, query);
      }).not.toThrow();
    });
  });

  describe('context evaluation', () => {
    test('should boost score for environment match', () => {
      const entries: KnowledgeEntry[] = [
        {
          id: 'prod-optimized',
          category: 'dockerfile',
          pattern: 'FROM',
          recommendation: 'Use production optimizations',
          tags: ['production', 'alpine']
        }
      ];

      const query: KnowledgeQuery = {
        environment: 'production',
        text: 'FROM node:16'
      };

      const matches = findKnowledgeMatches(entries, query);
      
      expect(matches[0].score).toBeGreaterThan(0); // Should have environment boost
      expect(matches[0].reasons.some(r => r.includes('Environment'))).toBe(true);
    });

    test('should boost score for framework match', () => {
      const entries: KnowledgeEntry[] = [
        {
          id: 'express-optimized',
          category: 'dockerfile',
          pattern: 'FROM',
          recommendation: 'Express optimization',
          tags: ['express', 'node']
        }
      ];

      const query: KnowledgeQuery = {
        framework: 'express',
        text: 'FROM node:16'
      };

      const matches = findKnowledgeMatches(entries, query);
      
      expect(matches[0].reasons.some(r => r.includes('Framework'))).toBe(true);
    });
  });
});