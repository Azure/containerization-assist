/**
 * Deterministic parsers for repository configuration files
 *
 * Each parser reads a project configuration file and extracts:
 * - Language and version
 * - Framework and version
 * - Dependencies
 * - Build system
 * - Default ports
 * - Entry points
 */

import { promises as fs } from 'node:fs';
import * as toml from '@iarna/toml';
import { parseStringPromise } from 'xml2js';
import { FRAMEWORK_PORTS } from '@/config/constants';

export interface ParsedConfig {
  language?: 'java' | 'dotnet' | 'javascript' | 'typescript' | 'python' | 'rust' | 'go' | 'other';
  framework?: string;
  frameworkVersion?: string;
  languageVersion?: string;
  dependencies?: string[];
  ports?: number[];
  entryPoint?: string;
  buildSystem?: {
    type: string;
    version?: string;
  };
}

/**
 * Parse package.json for Node.js projects
 */
export async function parsePackageJson(filePath: string): Promise<ParsedConfig> {
  try {
    const content = await fs.readFile(filePath, 'utf-8');
    const pkg = JSON.parse(content);

    let framework: string | undefined;
    const deps = { ...pkg.dependencies, ...pkg.devDependencies };

    if (deps['express']) framework = 'express';
    else if (deps['@nestjs/core']) framework = 'nestjs';
    else if (deps['next']) framework = 'next';
    else if (deps['nuxt']) framework = 'nuxt';
    else if (deps['react']) framework = 'react';
    else if (deps['vue']) framework = 'vue';
    else if (deps['@angular/core']) framework = 'angular';

    const ports: number[] = [];
    if (pkg.scripts) {
      // Look for PORT= or --port patterns
      const scriptStr = JSON.stringify(pkg.scripts);
      const portMatches = scriptStr.match(/(?:PORT=|--port[=\s]+)(\d+)/g);
      if (portMatches) {
        portMatches.forEach((match) => {
          const portMatch = match.match(/\d+/);
          if (portMatch) {
            const port = parseInt(portMatch[0], 10);
            if (port > 0 && port < 65536) ports.push(port);
          }
        });
      }
    }

    if (ports.length === 0) {
      if (framework === 'next' || framework === 'react') ports.push(FRAMEWORK_PORTS.next);
      else if (framework === 'angular') ports.push(FRAMEWORK_PORTS.angular);
      else if (framework === 'nuxt' || framework === 'vue') ports.push(FRAMEWORK_PORTS.nuxt);
      else ports.push(FRAMEWORK_PORTS.next); // Default Node.js port
    }

    let entryPoint = pkg.main || 'index.js';
    if (pkg.scripts?.start) {
      const startScript = pkg.scripts.start;
      // Extract entry point from "node server.js" or "ts-node src/index.ts"
      const match = startScript.match(/(?:node|ts-node)\s+(.+?)(?:\s|$)/);
      if (match) entryPoint = match[1];
    }

    const buildSystem = {
      type: 'npm',
      version: pkg.engines?.node,
    };

    const isTypeScript = !!(
      deps['typescript'] ||
      deps['ts-node'] ||
      deps['@typescript-eslint/parser']
    );

    const result: ParsedConfig = {
      language: isTypeScript ? 'typescript' : 'javascript',
      dependencies: Object.keys(deps).slice(0, 20), // Top 20 deps
      ports,
      entryPoint,
      buildSystem,
    };
    if (framework) result.framework = framework;
    if (pkg.engines?.node) result.languageVersion = pkg.engines.node;
    return result;
  } catch (error) {
    throw new Error(`Failed to parse package.json: ${error}`);
  }
}

/**
 * Parse pom.xml for Java/Maven projects
 */
export async function parsePomXml(filePath: string): Promise<ParsedConfig> {
  try {
    const content = await fs.readFile(filePath, 'utf-8');
    const pom = await parseStringPromise(content);

    const project = pom.project;
    const artifactId = project.artifactId?.[0] || '';
    const version = project.version?.[0] || '';

    let framework: string | undefined;
    const dependencies = project.dependencies?.[0]?.dependency || [];

    for (const dep of dependencies) {
      const depGroupId = dep.groupId?.[0] || '';
      const depArtifactId = dep.artifactId?.[0] || '';

      if (depGroupId.includes('springframework.boot')) {
        framework = 'spring-boot';
        break;
      } else if (depArtifactId.includes('quarkus')) {
        framework = 'quarkus';
        break;
      } else if (depArtifactId.includes('micronaut')) {
        framework = 'micronaut';
        break;
      } else if (depGroupId.includes('jakarta.ee')) {
        framework = 'jakarta-ee';
        break;
      }
    }

    const properties = project.properties?.[0] || {};
    const javaVersion =
      properties['java.version']?.[0] ||
      properties['maven.compiler.source']?.[0] ||
      properties['maven.compiler.target']?.[0];

    const ports: number[] = [];
    if (framework === 'spring-boot') ports.push(8080);
    else if (framework === 'quarkus') ports.push(8080);
    else if (framework === 'micronaut') ports.push(8080);
    else ports.push(8080); // Default Java port

    const entryPoint = `${artifactId}-${version}.jar`;

    const result: ParsedConfig = {
      language: 'java',
      dependencies: dependencies
        .map((d: unknown) => {
          const dep = d as { groupId?: string[]; artifactId?: string[] };
          return `${dep.groupId?.[0] || ''}:${dep.artifactId?.[0] || ''}`;
        })
        .slice(0, 20),
      ports,
      entryPoint,
      buildSystem: {
        type: 'maven',
      },
    };
    if (framework) result.framework = framework;
    if (javaVersion) result.languageVersion = javaVersion;
    if (project.modelVersion?.[0] && result.buildSystem) {
      result.buildSystem.version = project.modelVersion[0];
    }
    return result;
  } catch (error) {
    throw new Error(`Failed to parse pom.xml: ${error}`);
  }
}

/**
 * Parse build.gradle(.kts) for Java/Gradle projects
 */
export async function parseGradle(filePath: string): Promise<ParsedConfig> {
  try {
    const content = await fs.readFile(filePath, 'utf-8');

    let framework: string | undefined;
    if (content.includes('org.springframework.boot')) framework = 'spring-boot';
    else if (content.includes('io.quarkus')) framework = 'quarkus';
    else if (content.includes('io.micronaut')) framework = 'micronaut';

    let javaVersion: string | undefined;

    const toolchainMatch = content.match(/languageVersion\s*=\s*JavaLanguageVersion\.of\((\d+)\)/);
    if (toolchainMatch) {
      javaVersion = toolchainMatch[1];
    }

    if (!javaVersion) {
      const sourceCompatMatch = content.match(/sourceCompatibility\s*=\s*['"]?(\d+)['"]?/);
      if (sourceCompatMatch) javaVersion = sourceCompatMatch[1];
    }

    if (!javaVersion) {
      const targetCompatMatch = content.match(/targetCompatibility\s*=\s*['"]?(\d+)['"]?/);
      if (targetCompatMatch) javaVersion = targetCompatMatch[1];
    }

    // Extract dependencies (limited to first 20)
    const dependencies: string[] = [];
    const depPattern = /implementation\s+['"]([^'"]+)['"]/g;
    let match;
    while ((match = depPattern.exec(content)) !== null) {
      if (match[1]) dependencies.push(match[1]);
      if (dependencies.length >= 20) break;
    }

    const result: ParsedConfig = {
      language: 'java',
      dependencies,
      ports: framework === 'spring-boot' ? [8080] : [8080],
      buildSystem: {
        type: 'gradle',
      },
    };
    if (framework) result.framework = framework;
    if (javaVersion) result.languageVersion = javaVersion;
    return result;
  } catch (error) {
    throw new Error(`Failed to parse build.gradle: ${error}`);
  }
}

/**
 * Parse requirements.txt / pyproject.toml for Python projects
 */
export async function parsePythonConfig(filePath: string): Promise<ParsedConfig> {
  try {
    const content = await fs.readFile(filePath, 'utf-8');
    const isPyProject = filePath.endsWith('pyproject.toml');

    let framework: string | undefined;
    let dependencies: string[] = [];
    let pythonVersion: string | undefined;

    if (isPyProject) {
      // Parse TOML
      const parsed = toml.parse(content) as {
        project?: { dependencies?: string[]; requires_python?: string };
      };
      const project = parsed.project || {};

      dependencies = project.dependencies || [];
      pythonVersion = project.requires_python;

      const depStr = dependencies.join(' ').toLowerCase();
      if (depStr.includes('django')) framework = 'django';
      else if (depStr.includes('flask')) framework = 'flask';
      else if (depStr.includes('fastapi')) framework = 'fastapi';
      else if (depStr.includes('tornado')) framework = 'tornado';
    } else {
      // Parse requirements.txt (simple line-by-line)
      dependencies = content
        .split('\n')
        .map((line) => line.trim())
        .filter((line) => line && !line.startsWith('#'))
        .slice(0, 20);

      const depStr = dependencies.join(' ').toLowerCase();
      if (depStr.includes('django')) framework = 'django';
      else if (depStr.includes('flask')) framework = 'flask';
      else if (depStr.includes('fastapi')) framework = 'fastapi';
    }

    const ports: number[] = [];
    if (framework === 'django') ports.push(FRAMEWORK_PORTS.django);
    else if (framework === 'flask') ports.push(FRAMEWORK_PORTS.flask);
    else if (framework === 'fastapi') ports.push(FRAMEWORK_PORTS.fastapi);
    else ports.push(FRAMEWORK_PORTS.django);

    const result: ParsedConfig = {
      language: 'python',
      dependencies: dependencies.slice(0, 20),
      ports,
      buildSystem: {
        type: isPyProject ? 'poetry' : 'pip',
      },
    };
    if (framework) result.framework = framework;
    if (pythonVersion) result.languageVersion = pythonVersion;
    return result;
  } catch (error) {
    throw new Error(`Failed to parse Python config: ${error}`);
  }
}

/**
 * Parse Cargo.toml for Rust projects
 */
export async function parseCargoToml(filePath: string): Promise<ParsedConfig> {
  try {
    const content = await fs.readFile(filePath, 'utf-8');
    const parsed = toml.parse(content) as {
      package?: { 'rust-version'?: string; edition?: string };
      dependencies?: Record<string, unknown>;
    };

    const package_ = parsed.package || {};
    const dependencies = Object.keys(parsed.dependencies || {});

    let framework: string | undefined;
    if (dependencies.includes('actix-web')) framework = 'actix-web';
    else if (dependencies.includes('rocket')) framework = 'rocket';
    else if (dependencies.includes('warp')) framework = 'warp';
    else if (dependencies.includes('axum')) framework = 'axum';

    const rustVersion = package_['rust-version'] || package_.edition;
    const ports = framework ? [8080] : [];

    const result: ParsedConfig = {
      language: 'rust',
      dependencies: dependencies.slice(0, 20),
      ports,
      buildSystem: {
        type: 'cargo',
      },
    };
    if (framework) result.framework = framework;
    if (rustVersion) result.languageVersion = rustVersion;
    return result;
  } catch (error) {
    throw new Error(`Failed to parse Cargo.toml: ${error}`);
  }
}

/**
 * Parse .csproj for .NET projects
 */
export async function parseCsProj(filePath: string): Promise<ParsedConfig> {
  try {
    const content = await fs.readFile(filePath, 'utf-8');
    const csproj = await parseStringPromise(content);

    const project = csproj.Project;
    const propertyGroup = project.PropertyGroup?.[0] || {};
    const itemGroups = project.ItemGroup || [];

    const targetFramework = propertyGroup.TargetFramework?.[0];
    const dependencies: string[] = [];
    for (const itemGroup of itemGroups) {
      const packageRefs = itemGroup.PackageReference || [];
      for (const ref of packageRefs) {
        const name = ref.$?.Include;
        if (name) dependencies.push(name);
      }
    }

    let framework: string | undefined;
    const depStr = dependencies.join(' ');
    if (depStr.includes('Microsoft.AspNetCore')) framework = 'aspnet-core';
    else if (depStr.includes('Microsoft.EntityFrameworkCore')) framework = 'entity-framework';

    const ports =
      framework === 'aspnet-core'
        ? [FRAMEWORK_PORTS['aspnet-core'], FRAMEWORK_PORTS['aspnet-core-https']]
        : [FRAMEWORK_PORTS['aspnet-core']];

    const result: ParsedConfig = {
      language: 'dotnet',
      dependencies: dependencies.slice(0, 20),
      ports,
      buildSystem: {
        type: 'dotnet',
      },
    };
    if (framework) result.framework = framework;
    if (targetFramework) result.languageVersion = targetFramework;
    return result;
  } catch (error) {
    throw new Error(`Failed to parse .csproj: ${error}`);
  }
}

/**
 * Parse go.mod for Go projects
 */
export async function parseGoMod(filePath: string): Promise<ParsedConfig> {
  try {
    const content = await fs.readFile(filePath, 'utf-8');

    const goVersionMatch = content.match(/^go\s+(\d+\.\d+)/m);
    const goVersion = goVersionMatch?.[1];

    const dependencies: string[] = [];
    const requireMatches = content.match(/require\s+\(([^)]+)\)/s);
    if (requireMatches?.[1]) {
      const requireBlock = requireMatches[1];
      const lines = requireBlock.split('\n');
      for (const line of lines) {
        const match = line.trim().match(/^([^\s]+)/);
        if (match?.[1]) dependencies.push(match[1]);
        if (dependencies.length >= 20) break;
      }
    }

    let framework: string | undefined;
    const depStr = dependencies.join(' ');
    if (depStr.includes('gin-gonic/gin')) framework = 'gin';
    else if (depStr.includes('labstack/echo')) framework = 'echo';
    else if (depStr.includes('gofiber/fiber')) framework = 'fiber';
    else if (depStr.includes('gorilla/mux')) framework = 'gorilla';

    const result: ParsedConfig = {
      language: 'go',
      dependencies,
      ports: [8080],
      buildSystem: {
        type: 'go',
      },
    };
    if (framework) result.framework = framework;
    if (goVersion) result.languageVersion = goVersion;
    return result;
  } catch (error) {
    throw new Error(`Failed to parse go.mod: ${error}`);
  }
}
