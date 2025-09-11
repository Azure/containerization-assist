/**
 * Golden File Loader
 * Utilities for loading and managing golden files for regression testing
 */

import { promises as fs } from 'fs';
import * as path from 'path';
import { Result, Success, Failure } from '../../../src/types';

export interface GoldenFileMetadata {
  version: string;
  description: string;
  lastUpdated: string;
  tools: Record<string, ToolGoldenFileInfo>;
  workflows: Record<string, WorkflowGoldenFileInfo>;
}

export interface ToolGoldenFileInfo {
  description: string;
  variants: string[];
  fixtures: string[];
}

export interface WorkflowGoldenFileInfo {
  description: string;
  steps: string[];
  fixtures: string[];
}

export interface GoldenFileLoadOptions {
  variant?: string;
  fixture?: string;
  strict?: boolean; // Fail if file doesn't exist vs return null
}

export class GoldenFileLoader {
  private readonly basePath: string;
  private metadata?: GoldenFileMetadata;

  constructor(basePath: string = path.join(__dirname, 'golden')) {
    this.basePath = basePath;
  }

  /**
   * Load golden file metadata
   */
  async loadMetadata(): Promise<Result<GoldenFileMetadata>> {
    try {
      const metadataPath = path.join(this.basePath, 'metadata.json');
      const content = await fs.readFile(metadataPath, 'utf-8');
      this.metadata = JSON.parse(content);
      return Success(this.metadata);
    } catch (error) {
      return Failure(`Failed to load golden file metadata: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Load golden file for a specific tool
   */
  async loadToolGoldenFile<T = unknown>(
    toolName: string, 
    fixture: string,
    options: GoldenFileLoadOptions = {}
  ): Promise<Result<T | null>> {
    const { variant, strict = true } = options;
    
    try {
      // Build path: tools/{toolName}/{variant?}/{fixture}.json
      const pathParts = ['tools', toolName];
      if (variant) {
        pathParts.push(variant);
      }
      pathParts.push(`${fixture}.json`);
      
      const filePath = path.join(this.basePath, ...pathParts);
      
      try {
        const content = await fs.readFile(filePath, 'utf-8');
        const parsed = JSON.parse(content) as T;
        return Success(parsed);
      } catch (error) {
        if (!strict && (error as NodeJS.ErrnoException).code === 'ENOENT') {
          return Success(null);
        }
        throw error;
      }
    } catch (error) {
      return Failure(`Failed to load golden file for ${toolName}/${fixture}: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Load golden file for a workflow
   */
  async loadWorkflowGoldenFile<T = unknown>(
    workflowName: string,
    fixture: string,
    options: GoldenFileLoadOptions = {}
  ): Promise<Result<T | null>> {
    const { strict = true } = options;
    
    try {
      const filePath = path.join(this.basePath, 'workflows', `${workflowName}-${fixture}.json`);
      
      try {
        const content = await fs.readFile(filePath, 'utf-8');
        const parsed = JSON.parse(content) as T;
        return Success(parsed);
      } catch (error) {
        if (!strict && (error as NodeJS.ErrnoException).code === 'ENOENT') {
          return Success(null);
        }
        throw error;
      }
    } catch (error) {
      return Failure(`Failed to load workflow golden file ${workflowName}/${fixture}: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Save golden file (for updating/creating new golden files)
   */
  async saveToolGoldenFile<T>(
    toolName: string,
    fixture: string,
    data: T,
    options: GoldenFileLoadOptions = {}
  ): Promise<Result<void>> {
    const { variant } = options;
    
    try {
      const pathParts = ['tools', toolName];
      if (variant) {
        pathParts.push(variant);
      }
      
      const dir = path.join(this.basePath, ...pathParts);
      await fs.mkdir(dir, { recursive: true });
      
      const filePath = path.join(dir, `${fixture}.json`);
      const content = JSON.stringify(data, null, 2);
      await fs.writeFile(filePath, content, 'utf-8');
      
      return Success(undefined);
    } catch (error) {
      return Failure(`Failed to save golden file for ${toolName}/${fixture}: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * List available golden files for a tool
   */
  async listToolGoldenFiles(toolName: string, variant?: string): Promise<Result<string[]>> {
    try {
      const pathParts = ['tools', toolName];
      if (variant) {
        pathParts.push(variant);
      }
      
      const dir = path.join(this.basePath, ...pathParts);
      
      try {
        const files = await fs.readdir(dir);
        const goldenFiles = files
          .filter(file => file.endsWith('.json'))
          .map(file => file.replace('.json', ''));
        
        return Success(goldenFiles);
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code === 'ENOENT') {
          return Success([]);
        }
        throw error;
      }
    } catch (error) {
      return Failure(`Failed to list golden files for ${toolName}: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Validate that a golden file exists
   */
  async validateGoldenFile(toolName: string, fixture: string, variant?: string): Promise<Result<boolean>> {
    const result = await this.loadToolGoldenFile(toolName, fixture, { variant, strict: false });
    if (result.success) {
      return Success(result.data !== null);
    } else {
      return result;
    }
  }

  /**
   * Get tool information from metadata
   */
  async getToolInfo(toolName: string): Promise<Result<ToolGoldenFileInfo | null>> {
    if (!this.metadata) {
      const metadataResult = await this.loadMetadata();
      if (!metadataResult.success) {
        return metadataResult;
      }
    }

    const toolInfo = this.metadata!.tools[toolName];
    return Success(toolInfo || null);
  }

  /**
   * Get workflow information from metadata
   */
  async getWorkflowInfo(workflowName: string): Promise<Result<WorkflowGoldenFileInfo | null>> {
    if (!this.metadata) {
      const metadataResult = await this.loadMetadata();
      if (!metadataResult.success) {
        return metadataResult;
      }
    }

    const workflowInfo = this.metadata!.workflows[workflowName];
    return Success(workflowInfo || null);
  }
}

/**
 * Global golden file loader instance
 */
export const goldenFileLoader = new GoldenFileLoader();

/**
 * Convenience functions for common operations
 */
export async function loadGoldenFile<T>(
  toolName: string, 
  fixture: string, 
  variant?: string
): Promise<T | null> {
  const result = await goldenFileLoader.loadToolGoldenFile<T>(toolName, fixture, { variant, strict: false });
  return result.success ? result.data : null;
}

export async function loadWorkflowGoldenFile<T>(
  workflowName: string,
  fixture: string
): Promise<T | null> {
  const result = await goldenFileLoader.loadWorkflowGoldenFile<T>(workflowName, fixture, { strict: false });
  return result.success ? result.data : null;
}

export async function saveGoldenFile<T>(
  toolName: string,
  fixture: string,
  data: T,
  variant?: string
): Promise<boolean> {
  const result = await goldenFileLoader.saveToolGoldenFile(toolName, fixture, data, { variant });
  return result.success;
}