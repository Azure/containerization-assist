/**
 * Fixture Registry System
 * Centralized management and discovery of test fixtures
 */

import { promises as fs } from 'fs';
import * as path from 'path';
import { Result, Success, Failure } from '../../../src/types';
import { EnvironmentCapabilities } from '../utilities/environment-detector';

export interface FixtureMetadata {
  id: string;
  name: string;
  type: 'project' | 'golden' | 'mock' | 'k8s' | 'docker' | 'workflow';
  category?: string; // e.g., 'java', 'node', 'security', etc.
  tags: string[];
  description: string;
  requirements?: Array<keyof Omit<EnvironmentCapabilities, 'platform'>>;
  version: string;
  lastUpdated: Date;
  path: string;
  relatedFixtures?: string[]; // IDs of related fixtures
  variants?: string[]; // Available variants of this fixture
}

export interface FixtureSearchCriteria {
  type?: FixtureMetadata['type'];
  category?: string;
  tags?: string[];
  requirements?: Array<keyof Omit<EnvironmentCapabilities, 'platform'>>;
  hasVariants?: boolean;
}

export interface FixtureValidationResult {
  valid: boolean;
  errors: string[];
  warnings: string[];
  metadata?: FixtureMetadata;
}

/**
 * Centralized fixture registry for managing test data
 */
export class FixtureRegistry {
  private registry = new Map<string, FixtureMetadata>();
  private basePath: string;
  private initialized = false;

  constructor(basePath: string = path.join(__dirname)) {
    this.basePath = basePath;
  }

  /**
   * Initialize the registry by scanning for fixtures
   */
  async initialize(): Promise<Result<void>> {
    if (this.initialized) {
      return Success(undefined);
    }

    try {
      await this.scanForFixtures();
      this.initialized = true;
      return Success(undefined);
    } catch (error) {
      return Failure(`Failed to initialize fixture registry: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Register a fixture manually
   */
  register(fixture: FixtureMetadata): void {
    this.registry.set(fixture.id, fixture);
  }

  /**
   * Find fixtures matching criteria
   */
  find(criteria: FixtureSearchCriteria): FixtureMetadata[] {
    const fixtures = Array.from(this.registry.values());

    return fixtures.filter(fixture => {
      // Type filter
      if (criteria.type && fixture.type !== criteria.type) {
        return false;
      }

      // Category filter
      if (criteria.category && fixture.category !== criteria.category) {
        return false;
      }

      // Tags filter (all tags must be present)
      if (criteria.tags && criteria.tags.length > 0) {
        const hasAllTags = criteria.tags.every(tag => fixture.tags.includes(tag));
        if (!hasAllTags) {
          return false;
        }
      }

      // Requirements filter (fixture must not require unavailable services)
      if (criteria.requirements && fixture.requirements) {
        const hasRequiredServices = fixture.requirements.every(req => 
          criteria.requirements!.includes(req)
        );
        if (!hasRequiredServices) {
          return false;
        }
      }

      // Variants filter
      if (criteria.hasVariants !== undefined) {
        const hasVariants = fixture.variants && fixture.variants.length > 0;
        if (criteria.hasVariants !== hasVariants) {
          return false;
        }
      }

      return true;
    });
  }

  /**
   * Load fixture data by ID
   */
  async load<T>(id: string): Promise<Result<T>> {
    const fixture = this.registry.get(id);
    if (!fixture) {
      return Failure(`Fixture not found: ${id}`);
    }

    try {
      const fullPath = path.resolve(this.basePath, fixture.path);
      
      // Handle different fixture types
      switch (fixture.type) {
        case 'golden':
          const content = await fs.readFile(fullPath, 'utf-8');
          const parsed = JSON.parse(content) as T;
          return Success(parsed);

        case 'project':
          // For project fixtures, return directory contents info
          const stats = await fs.stat(fullPath);
          if (stats.isDirectory()) {
            const files = await this.readDirectoryRecursive(fullPath);
            return Success({ path: fullPath, files } as T);
          } else {
            const fileContent = await fs.readFile(fullPath, 'utf-8');
            return Success({ path: fullPath, content: fileContent } as T);
          }

        case 'k8s':
        case 'docker':
          const yamlContent = await fs.readFile(fullPath, 'utf-8');
          return Success({ content: yamlContent, path: fullPath } as T);

        case 'mock':
        case 'workflow':
          // These are typically JSON configurations
          const jsonContent = await fs.readFile(fullPath, 'utf-8');
          const jsonParsed = JSON.parse(jsonContent) as T;
          return Success(jsonParsed);

        default:
          return Failure(`Unsupported fixture type: ${fixture.type}`);
      }
    } catch (error) {
      return Failure(`Failed to load fixture ${id}: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Load fixture with variant
   */
  async loadVariant<T>(id: string, variant: string): Promise<Result<T>> {
    const fixture = this.registry.get(id);
    if (!fixture) {
      return Failure(`Fixture not found: ${id}`);
    }

    if (!fixture.variants || !fixture.variants.includes(variant)) {
      return Failure(`Variant '${variant}' not found for fixture ${id}`);
    }

    try {
      // Construct variant path
      const baseName = path.basename(fixture.path, path.extname(fixture.path));
      const extension = path.extname(fixture.path);
      const dir = path.dirname(fixture.path);
      const variantPath = path.join(dir, 'variants', `${baseName}-${variant}${extension}`);
      const fullPath = path.resolve(this.basePath, variantPath);

      const content = await fs.readFile(fullPath, 'utf-8');
      
      if (extension === '.json') {
        const parsed = JSON.parse(content) as T;
        return Success(parsed);
      } else {
        return Success({ content, path: fullPath } as T);
      }
    } catch (error) {
      return Failure(`Failed to load variant ${variant} for fixture ${id}: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Validate fixture integrity
   */
  async validate(id: string): Promise<FixtureValidationResult> {
    const fixture = this.registry.get(id);
    if (!fixture) {
      return {
        valid: false,
        errors: [`Fixture not found: ${id}`],
        warnings: []
      };
    }

    const result: FixtureValidationResult = {
      valid: true,
      errors: [],
      warnings: [],
      metadata: fixture
    };

    try {
      const fullPath = path.resolve(this.basePath, fixture.path);
      
      // Check if path exists
      try {
        await fs.access(fullPath);
      } catch {
        result.valid = false;
        result.errors.push(`Fixture path does not exist: ${fullPath}`);
        return result;
      }

      // Validate based on type
      switch (fixture.type) {
        case 'golden':
        case 'mock':
        case 'workflow':
          try {
            const content = await fs.readFile(fullPath, 'utf-8');
            JSON.parse(content); // Validate JSON syntax
          } catch (error) {
            result.valid = false;
            result.errors.push(`Invalid JSON in fixture: ${error instanceof Error ? error.message : 'Parse error'}`);
          }
          break;

        case 'project':
          const stats = await fs.stat(fullPath);
          if (stats.isDirectory()) {
            // Check for common project files
            const files = await fs.readdir(fullPath);
            if (files.length === 0) {
              result.warnings.push('Project directory is empty');
            }
          }
          break;
      }

      // Validate variants if specified
      if (fixture.variants && fixture.variants.length > 0) {
        for (const variant of fixture.variants) {
          try {
            await this.loadVariant(id, variant);
          } catch (error) {
            result.warnings.push(`Variant '${variant}' could not be loaded: ${error instanceof Error ? error.message : 'Unknown error'}`);
          }
        }
      }

      // Validate related fixtures
      if (fixture.relatedFixtures && fixture.relatedFixtures.length > 0) {
        for (const relatedId of fixture.relatedFixtures) {
          if (!this.registry.has(relatedId)) {
            result.warnings.push(`Related fixture not found: ${relatedId}`);
          }
        }
      }

    } catch (error) {
      result.valid = false;
      result.errors.push(`Validation error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }

    return result;
  }

  /**
   * Get all registered fixture IDs
   */
  getAllFixtureIds(): string[] {
    return Array.from(this.registry.keys());
  }

  /**
   * Get fixture metadata by ID
   */
  getMetadata(id: string): FixtureMetadata | null {
    return this.registry.get(id) || null;
  }

  /**
   * Filter fixtures by environment capabilities
   */
  getAvailableFixtures(capabilities: EnvironmentCapabilities): FixtureMetadata[] {
    return Array.from(this.registry.values()).filter(fixture => {
      if (!fixture.requirements) {
        return true; // No requirements, always available
      }

      return fixture.requirements.every(req => capabilities[req].available);
    });
  }

  /**
   * Private: Scan directories for fixtures
   */
  private async scanForFixtures(): Promise<void> {
    // Scan project fixtures
    await this.scanProjectFixtures();
    
    // Scan golden files
    await this.scanGoldenFiles();
    
    // Scan K8s manifests
    await this.scanK8sFixtures();
    
    // Scan Docker files
    await this.scanDockerFixtures();
    
    // Scan existing expected outputs
    await this.scanExpectedOutputs();
  }

  private async scanProjectFixtures(): Promise<void> {
    const projectsPath = path.join(this.basePath, 'projects');
    
    try {
      const languages = await fs.readdir(projectsPath);
      
      for (const lang of languages) {
        const langPath = path.join(projectsPath, lang);
        const stats = await fs.stat(langPath);
        
        if (stats.isDirectory()) {
          this.register({
            id: `project-${lang}`,
            name: `${lang.charAt(0).toUpperCase() + lang.slice(1)} Project`,
            type: 'project',
            category: lang,
            tags: ['project', lang],
            description: `Sample ${lang} project for testing`,
            version: '1.0.0',
            lastUpdated: stats.mtime,
            path: path.relative(this.basePath, langPath),
          });
        }
      }
    } catch (error) {
      // Projects directory may not exist, which is fine
    }

    // Also scan top-level project fixtures
    const topLevelProjects = [
      'java-spring-boot-maven',
      'node-express', 
      'dotnet-webapi',
      'python-flask'
    ];

    for (const project of topLevelProjects) {
      const projectPath = path.join(this.basePath, project);
      
      try {
        const stats = await fs.stat(projectPath);
        const category = this.inferProjectCategory(project);
        
        this.register({
          id: `project-${project}`,
          name: project.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' '),
          type: 'project',
          category,
          tags: ['project', category, ...project.split('-')],
          description: `${project} sample project`,
          version: '1.0.0',
          lastUpdated: stats.mtime,
          path: path.relative(this.basePath, projectPath),
        });
      } catch (error) {
        // Project may not exist
      }
    }
  }

  private async scanGoldenFiles(): Promise<void> {
    const goldenPath = path.join(this.basePath, 'golden');
    
    try {
      await this.scanGoldenDirectory(goldenPath, 'tools');
      await this.scanGoldenDirectory(goldenPath, 'workflows'); 
    } catch (error) {
      // Golden directory may not exist
    }
  }

  private async scanGoldenDirectory(goldenPath: string, subDir: string): Promise<void> {
    const dirPath = path.join(goldenPath, subDir);
    
    try {
      const items = await fs.readdir(dirPath);
      
      for (const item of items) {
        const itemPath = path.join(dirPath, item);
        const stats = await fs.stat(itemPath);
        
        if (stats.isDirectory() && subDir === 'tools') {
          // Tool golden files
          const files = await fs.readdir(itemPath);
          const goldenFiles = files.filter(f => f.endsWith('.json'));
          
          for (const file of goldenFiles) {
            const fixture = path.basename(file, '.json');
            const filePath = path.join(itemPath, file);
            const fileStats = await fs.stat(filePath);
            
            this.register({
              id: `golden-${item}-${fixture}`,
              name: `${item} Golden File - ${fixture}`,
              type: 'golden',
              category: item,
              tags: ['golden', item, fixture],
              description: `Expected output for ${item} tool with ${fixture} fixture`,
              version: '1.0.0',
              lastUpdated: fileStats.mtime,
              path: path.relative(this.basePath, filePath),
            });
          }
        } else if (item.endsWith('.json') && subDir === 'workflows') {
          // Workflow golden files
          const fileStats = await fs.stat(itemPath);
          const workflowName = path.basename(item, '.json');
          
          this.register({
            id: `golden-workflow-${workflowName}`,
            name: `Workflow Golden File - ${workflowName}`,
            type: 'golden',
            category: 'workflow',
            tags: ['golden', 'workflow', workflowName],
            description: `Expected output for ${workflowName} workflow`,
            version: '1.0.0',
            lastUpdated: fileStats.mtime,
            path: path.relative(this.basePath, itemPath),
          });
        }
      }
    } catch (error) {
      // Directory may not exist
    }
  }

  private async scanK8sFixtures(): Promise<void> {
    const k8sPath = path.join(this.basePath, 'k8s');
    
    try {
      const environments = await fs.readdir(k8sPath);
      
      for (const env of environments) {
        const envPath = path.join(k8sPath, env);
        const stats = await fs.stat(envPath);
        
        if (stats.isDirectory()) {
          const files = await fs.readdir(envPath);
          const yamlFiles = files.filter(f => f.endsWith('.yaml') || f.endsWith('.yml'));
          
          for (const file of yamlFiles) {
            const filePath = path.join(envPath, file);
            const fileStats = await fs.stat(filePath);
            const resourceName = path.basename(file, path.extname(file));
            
            this.register({
              id: `k8s-${env}-${resourceName}`,
              name: `K8s Manifest - ${resourceName} (${env})`,
              type: 'k8s',
              category: env,
              tags: ['k8s', 'kubernetes', env, resourceName],
              description: `Kubernetes manifest for ${resourceName} in ${env} environment`,
              requirements: [], // K8s manifests don't require running cluster for loading
              version: '1.0.0',
              lastUpdated: fileStats.mtime,
              path: path.relative(this.basePath, filePath),
            });
          }
        }
      }
    } catch (error) {
      // K8s directory may not exist
    }
  }

  private async scanDockerFixtures(): Promise<void> {
    const dockerPath = path.join(this.basePath, 'dockerfiles');
    
    try {
      const files = await fs.readdir(dockerPath);
      const dockerFiles = files.filter(f => f.includes('Dockerfile') || f.endsWith('.dockerfile'));
      
      for (const file of dockerFiles) {
        const filePath = path.join(dockerPath, file);
        const stats = await fs.stat(filePath);
        const dockerfileName = path.basename(file, path.extname(file));
        
        this.register({
          id: `docker-${dockerfileName}`,
          name: `Dockerfile - ${dockerfileName}`,
          type: 'docker',
          category: 'dockerfile',
          tags: ['docker', 'dockerfile', dockerfileName],
          description: `Dockerfile fixture for ${dockerfileName}`,
          version: '1.0.0',
          lastUpdated: stats.mtime,
          path: path.relative(this.basePath, filePath),
        });
      }
    } catch (error) {
      // Docker directory may not exist
    }
  }

  private async scanExpectedOutputs(): Promise<void> {
    const outputsPath = path.join(this.basePath, 'expected-outputs');
    
    try {
      const files = await fs.readdir(outputsPath);
      const jsonFiles = files.filter(f => f.endsWith('.json'));
      
      for (const file of jsonFiles) {
        const filePath = path.join(outputsPath, file);
        const stats = await fs.stat(filePath);
        const outputName = path.basename(file, '.json');
        
        this.register({
          id: `output-${outputName}`,
          name: `Expected Output - ${outputName}`,
          type: 'golden',
          category: 'expected-output',
          tags: ['golden', 'expected-output', outputName],
          description: `Expected output for ${outputName}`,
          version: '1.0.0',
          lastUpdated: stats.mtime,
          path: path.relative(this.basePath, filePath),
        });
      }
    } catch (error) {
      // Expected outputs directory may not exist
    }
  }

  private async readDirectoryRecursive(dirPath: string): Promise<string[]> {
    const files: string[] = [];
    const items = await fs.readdir(dirPath);
    
    for (const item of items) {
      const itemPath = path.join(dirPath, item);
      const stats = await fs.stat(itemPath);
      
      if (stats.isDirectory()) {
        const subFiles = await this.readDirectoryRecursive(itemPath);
        files.push(...subFiles);
      } else {
        files.push(path.relative(dirPath, itemPath));
      }
    }
    
    return files;
  }

  private inferProjectCategory(projectName: string): string {
    if (projectName.includes('java') || projectName.includes('spring')) return 'java';
    if (projectName.includes('node') || projectName.includes('express')) return 'node';
    if (projectName.includes('dotnet') || projectName.includes('aspnet')) return 'dotnet';
    if (projectName.includes('python') || projectName.includes('flask')) return 'python';
    return 'unknown';
  }
}

/**
 * Global fixture registry instance
 */
export const fixtureRegistry = new FixtureRegistry();

/**
 * Convenience functions
 */
export async function findFixtures(criteria: FixtureSearchCriteria): Promise<FixtureMetadata[]> {
  const initResult = await fixtureRegistry.initialize();
  if (!initResult.success) {
    console.warn(`Failed to initialize fixture registry: ${initResult.error}`);
    return [];
  }
  
  return fixtureRegistry.find(criteria);
}

export async function loadFixture<T>(id: string, variant?: string): Promise<T | null> {
  const initResult = await fixtureRegistry.initialize();
  if (!initResult.success) {
    return null;
  }
  
  const result = variant 
    ? await fixtureRegistry.loadVariant<T>(id, variant)
    : await fixtureRegistry.load<T>(id);
    
  return result.success ? result.data : null;
}