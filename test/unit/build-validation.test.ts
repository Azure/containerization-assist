import { describe, it, expect, beforeAll } from '@jest/globals';
import { existsSync, readdirSync, statSync } from 'fs';
import { join } from 'path';
import { execSync } from 'child_process';

/**
 * Build Validation Tests
 * 
 * These tests ensure that critical runtime resources (prompts and knowledge data)
 * are properly included in the built package. This prevents issues where the
 * published npm package is missing required files.
 */
describe('Build Output Validation', () => {
  const rootDir = process.cwd();
  const distDir = join(rootDir, 'dist');
  const distCjsDir = join(rootDir, 'dist-cjs');

  // Build the project before running tests if dist doesn't exist
  beforeAll(() => {
    if (!existsSync(distDir)) {
      console.log('Building project for validation tests...');
      execSync('npm run build', { stdio: 'inherit' });
    }
  });

  describe('ESM Build (dist)', () => {
    describe('Prompts Directory', () => {
      const promptsDir = join(distDir, 'src', 'prompts');

      it('should include prompts directory in dist', () => {
        expect(existsSync(promptsDir)).toBe(true);
      });

      it('should include all prompt category directories', () => {
        const expectedCategories = [
          'analysis',
          'containerization',
          'orchestration',
          'sampling',
          'security',
          'validation'
        ];

        const categories = readdirSync(promptsDir).filter(item => {
          const itemPath = join(promptsDir, item);
          return statSync(itemPath).isDirectory();
        });

        expectedCategories.forEach(category => {
          expect(categories).toContain(category);
        });
      });

      it('should include JSON prompt files in each category', () => {
        const categories = readdirSync(promptsDir).filter(item => {
          const itemPath = join(promptsDir, item);
          return statSync(itemPath).isDirectory();
        });

        categories.forEach(category => {
          const categoryPath = join(promptsDir, category);
          const files = readdirSync(categoryPath);
          const jsonFiles = files.filter(f => f.endsWith('.json'));
          
          expect(jsonFiles.length).toBeGreaterThan(0);
        });
      });

      it('should include specific critical prompt files', () => {
        const criticalPrompts = [
          join(promptsDir, 'analysis', 'enhance-repo-analysis.json'),
          join(promptsDir, 'containerization', 'dockerfile-generation.json'),
          join(promptsDir, 'orchestration', 'k8s-manifest-generation.json')
        ];

        criticalPrompts.forEach(promptFile => {
          expect(existsSync(promptFile)).toBe(true);
        });
      });
    });

    describe('Knowledge Data Directory', () => {
      const knowledgeDataDir = join(distDir, 'src', 'knowledge', 'data');

      it('should include knowledge data directory in dist', () => {
        expect(existsSync(knowledgeDataDir)).toBe(true);
      });

      it('should include all knowledge pack files', () => {
        const expectedPacks = [
          'starter-pack.json',
          'nodejs-pack.json',
          'python-pack.json',
          'java-pack.json',
          'dotnet-pack.json',
          'go-pack.json',
          'kubernetes-pack.json',
          'security-pack.json'
        ];

        const files = readdirSync(knowledgeDataDir);
        
        expectedPacks.forEach(pack => {
          expect(files).toContain(pack);
        });
      });

      it('should have valid JSON content in knowledge files', () => {
        const files = readdirSync(knowledgeDataDir).filter(f => f.endsWith('.json'));
        
        files.forEach(file => {
          const filePath = join(knowledgeDataDir, file);
          expect(() => {
            require(filePath);
          }).not.toThrow();
        });
      });
    });
  });

  describe('CommonJS Build (dist-cjs)', () => {
    describe('Prompts Directory', () => {
      const promptsDir = join(distCjsDir, 'src', 'prompts');

      it('should include prompts directory in dist-cjs', () => {
        expect(existsSync(promptsDir)).toBe(true);
      });

      it('should include all prompt category directories', () => {
        const expectedCategories = [
          'analysis',
          'containerization',
          'orchestration',
          'sampling',
          'security',
          'validation'
        ];

        const categories = readdirSync(promptsDir).filter(item => {
          const itemPath = join(promptsDir, item);
          return statSync(itemPath).isDirectory();
        });

        expectedCategories.forEach(category => {
          expect(categories).toContain(category);
        });
      });

      it('should include JSON prompt files in CommonJS build', () => {
        const categories = readdirSync(promptsDir).filter(item => {
          const itemPath = join(promptsDir, item);
          return statSync(itemPath).isDirectory();
        });

        categories.forEach(category => {
          const categoryPath = join(promptsDir, category);
          const files = readdirSync(categoryPath);
          const jsonFiles = files.filter(f => f.endsWith('.json'));
          
          expect(jsonFiles.length).toBeGreaterThan(0);
        });
      });
    });

    describe('Knowledge Data Directory', () => {
      const knowledgeDataDir = join(distCjsDir, 'src', 'knowledge', 'data');

      it('should include knowledge data directory in dist-cjs', () => {
        expect(existsSync(knowledgeDataDir)).toBe(true);
      });

      it('should include all knowledge pack files in CommonJS build', () => {
        const expectedPacks = [
          'starter-pack.json',
          'nodejs-pack.json',
          'python-pack.json',
          'java-pack.json',
          'dotnet-pack.json',
          'go-pack.json',
          'kubernetes-pack.json',
          'security-pack.json'
        ];

        const files = readdirSync(knowledgeDataDir);
        
        expectedPacks.forEach(pack => {
          expect(files).toContain(pack);
        });
      });
    });
  });

  describe('Package Integrity', () => {
    it('should have consistent file counts between ESM and CommonJS builds', () => {
      const esmPromptsDir = join(distDir, 'src', 'prompts');
      const cjsPromptsDir = join(distCjsDir, 'src', 'prompts');
      
      const countFiles = (dir: string): number => {
        let count = 0;
        const items = readdirSync(dir);
        items.forEach(item => {
          const itemPath = join(dir, item);
          if (statSync(itemPath).isDirectory()) {
            count += countFiles(itemPath);
          } else if (item.endsWith('.json')) {
            count++;
          }
        });
        return count;
      };

      const esmPromptCount = countFiles(esmPromptsDir);
      const cjsPromptCount = countFiles(cjsPromptsDir);
      
      expect(esmPromptCount).toBe(cjsPromptCount);
      expect(esmPromptCount).toBeGreaterThan(0);
    });

    it('should have knowledge data files with reasonable sizes', () => {
      const knowledgeDataDir = join(distDir, 'src', 'knowledge', 'data');
      const files = readdirSync(knowledgeDataDir).filter(f => f.endsWith('.json'));
      
      files.forEach(file => {
        const filePath = join(knowledgeDataDir, file);
        const stats = statSync(filePath);
        
        // Each knowledge pack should be at least 1KB but not more than 100KB
        expect(stats.size).toBeGreaterThan(1000);
        expect(stats.size).toBeLessThan(100000);
      });
    });
  });

  describe('Runtime Loading Validation', () => {
    it('should be able to find prompts directory at runtime', () => {
      // Since we're testing the build output, we can verify the helper module exists
      const helperPath = join(distDir, 'src', 'lib', 'find-prompts-dir.js');
      expect(existsSync(helperPath)).toBe(true);
      
      // Verify prompts can be found in expected locations
      const possiblePromptDirs = [
        join(rootDir, 'src', 'prompts'),
        join(distDir, 'src', 'prompts'),
        join(distCjsDir, 'src', 'prompts')
      ];
      
      const foundPromptDir = possiblePromptDirs.find(dir => existsSync(dir));
      expect(foundPromptDir).toBeDefined();
      
      // Verify it contains expected structure
      if (foundPromptDir) {
        const categories = readdirSync(foundPromptDir).filter(item => {
          const itemPath = join(foundPromptDir, item);
          return statSync(itemPath).isDirectory();
        });
        
        expect(categories.length).toBeGreaterThan(0);
      }
    });
  });
});