/**
 * Package.json parsing utilities for consistent parsing across the codebase
 */

import { promises as fs, readFileSync } from 'fs';
import { joinPaths } from '@/lib/path-utils';

/**
 * Parsed package.json structure
 */
export interface PackageJson {
  name?: string;
  version?: string;
  description?: string;
  main?: string;
  scripts?: Record<string, string>;
  dependencies?: Record<string, string>;
  devDependencies?: Record<string, string>;
  peerDependencies?: Record<string, string>;
  optionalDependencies?: Record<string, string>;
  engines?: {
    node?: string;
    npm?: string;
    yarn?: string;
    pnpm?: string;
  };
  type?: 'module' | 'commonjs';
  private?: boolean;
  workspaces?: string[] | { packages?: string[] };
  [key: string]: unknown;
}

/**
 * Parses package.json file from a directory
 *
 * @param dirPath - Directory containing package.json
 * @returns Parsed package.json object
 * @throws Error if file not found or invalid JSON
 */
export async function parsePackageJson(dirPath: string): Promise<PackageJson> {
  const packageJsonPath = joinPaths(dirPath, 'package.json');

  try {
    const content = await fs.readFile(packageJsonPath, 'utf-8');
    return JSON.parse(content) as PackageJson;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === 'ENOENT') {
      throw new Error(`No package.json found in ${dirPath}`);
    }

    if (error instanceof SyntaxError) {
      throw new Error(`Invalid JSON in package.json: ${error.message}`);
    }

    throw error;
  }
}

/**
 * Synchronously parses package.json file from a directory
 */
export function parsePackageJsonSync(dirPath: string): PackageJson {
  const packageJsonPath = joinPaths(dirPath, 'package.json');

  try {
    const content = readFileSync(packageJsonPath, 'utf-8');
    return JSON.parse(content) as PackageJson;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === 'ENOENT') {
      throw new Error(`No package.json found in ${dirPath}`);
    }

    if (error instanceof SyntaxError) {
      throw new Error(`Invalid JSON in package.json: ${error.message}`);
    }

    throw error;
  }
}

/**
 * Checks if a package.json exists in a directory
 */
export async function hasPackageJson(dirPath: string): Promise<boolean> {
  const packageJsonPath = joinPaths(dirPath, 'package.json');

  try {
    await fs.access(packageJsonPath);
    return true;
  } catch {
    return false;
  }
}

/**
 * Extracts all dependencies from package.json
 */
export function getAllDependencies(packageJson: PackageJson): Record<string, string> {
  return {
    ...packageJson.dependencies,
    ...packageJson.devDependencies,
    ...packageJson.peerDependencies,
    ...packageJson.optionalDependencies,
  };
}

/**
 * Detects the package manager based on lock files
 */
export async function detectPackageManager(
  dirPath: string,
): Promise<'npm' | 'yarn' | 'pnpm' | 'bun' | null> {
  const files = await fs.readdir(dirPath);

  if (files.includes('bun.lockb')) return 'bun';
  if (files.includes('pnpm-lock.yaml')) return 'pnpm';
  if (files.includes('yarn.lock')) return 'yarn';
  if (files.includes('package-lock.json')) return 'npm';

  return null;
}

/**
 * Checks if a specific dependency exists in package.json
 */
export function hasDependency(packageJson: PackageJson, dependencyName: string): boolean {
  const allDeps = getAllDependencies(packageJson);
  return dependencyName in allDeps;
}

/**
 * Gets the version of a specific dependency
 */
export function getDependencyVersion(
  packageJson: PackageJson,
  dependencyName: string,
): string | undefined {
  const allDeps = getAllDependencies(packageJson);
  return allDeps[dependencyName];
}

/**
 * Detects common frameworks from package.json
 */
export function detectFrameworks(packageJson: PackageJson): string[] {
  const frameworks: string[] = [];
  const deps = getAllDependencies(packageJson);

  // React ecosystem
  if ('react' in deps) frameworks.push('react');
  if ('next' in deps) frameworks.push('nextjs');
  if ('gatsby' in deps) frameworks.push('gatsby');

  // Vue ecosystem
  if ('vue' in deps) frameworks.push('vue');
  if ('nuxt' in deps) frameworks.push('nuxt');

  // Angular
  if ('@angular/core' in deps) frameworks.push('angular');

  // Node.js frameworks
  if ('express' in deps) frameworks.push('express');
  if ('fastify' in deps) frameworks.push('fastify');
  if ('koa' in deps) frameworks.push('koa');
  if ('nestjs' in deps || '@nestjs/core' in deps) frameworks.push('nestjs');

  // Testing frameworks
  if ('jest' in deps) frameworks.push('jest');
  if ('mocha' in deps) frameworks.push('mocha');
  if ('vitest' in deps) frameworks.push('vitest');

  // Build tools
  if ('webpack' in deps) frameworks.push('webpack');
  if ('vite' in deps) frameworks.push('vite');
  if ('rollup' in deps) frameworks.push('rollup');
  if ('esbuild' in deps) frameworks.push('esbuild');

  return frameworks;
}

/**
 * Checks if the project is a monorepo
 */
export function isMonorepo(packageJson: PackageJson): boolean {
  return (
    !!packageJson.workspaces ||
    hasDependency(packageJson, 'lerna') ||
    hasDependency(packageJson, '@nrwl/workspace') ||
    hasDependency(packageJson, 'rush')
  );
}
