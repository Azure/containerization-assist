/**
 * Test Fixtures Helper
 * Provides organized access to test data and sample projects
 */

import { join } from 'path';
import { readFile, access } from 'fs/promises';

export interface ProjectFixture {
  name: string;
  path: string;
  language: string;
  framework?: string;
  description: string;
  hasDockerfile: boolean;
  buildCommand?: string;
  startCommand?: string;
}

export interface ExpectedOutput {
  path: string;
  content: any;
}

export class ProjectFixtures {
  private static readonly FIXTURES_BASE = join(process.cwd(), 'test/__support__/fixtures');

  /**
   * Available project fixtures
   */
  static readonly PROJECTS: Record<string, ProjectFixture> = {
    'node-express': {
      name: 'node-express',
      path: join(ProjectFixtures.FIXTURES_BASE, 'node-express'),
      language: 'javascript',
      framework: 'express',
      description: 'Simple Node.js Express application',
      hasDockerfile: false,
      buildCommand: 'npm install',
      startCommand: 'npm start'
    },
    'java-spring-boot-maven': {
      name: 'java-spring-boot-maven',
      path: join(ProjectFixtures.FIXTURES_BASE, 'java-spring-boot-maven'),
      language: 'java',
      framework: 'spring-boot',
      description: 'Spring Boot application with Maven',
      hasDockerfile: false,
      buildCommand: 'mvn clean package',
      startCommand: 'java -jar target/*.jar'
    },
    'dotnet-webapi': {
      name: 'dotnet-webapi',
      path: join(ProjectFixtures.FIXTURES_BASE, 'dotnet-webapi'),
      language: 'csharp',
      framework: 'aspnet-core',
      description: '.NET Core Web API application',
      hasDockerfile: false,
      buildCommand: 'dotnet build',
      startCommand: 'dotnet run'
    },
    'python-flask': {
      name: 'python-flask',
      path: join(ProjectFixtures.FIXTURES_BASE, 'python-flask'),
      language: 'python',
      framework: 'flask',
      description: 'Python Flask web application',
      hasDockerfile: false,
      buildCommand: 'pip install -r requirements.txt',
      startCommand: 'python app.py'
    },
    'mcp-server-architecture': {
      name: 'mcp-server-architecture',
      path: join(ProjectFixtures.FIXTURES_BASE, 'mcp-server-architecture'),
      language: 'typescript',
      framework: 'mcp',
      description: 'MCP Server TypeScript architecture',
      hasDockerfile: false,
      buildCommand: 'npm install && npm run build',
      startCommand: 'npm start'
    }
  };

  /**
   * Get fixture path by name
   */
  static getPath(name: string): string {
    const fixture = this.PROJECTS[name];
    if (!fixture) {
      throw new Error(`Unknown fixture: ${name}. Available: ${Object.keys(this.PROJECTS).join(', ')}`);
    }
    return fixture.path;
  }

  /**
   * Get fixture metadata
   */
  static getFixture(name: string): ProjectFixture {
    const fixture = this.PROJECTS[name];
    if (!fixture) {
      throw new Error(`Unknown fixture: ${name}. Available: ${Object.keys(this.PROJECTS).join(', ')}`);
    }
    return fixture;
  }

  /**
   * Get all available fixture names
   */
  static getAvailableNames(): string[] {
    return Object.keys(this.PROJECTS);
  }

  /**
   * Get fixtures by language
   */
  static getByLanguage(language: string): ProjectFixture[] {
    return Object.values(this.PROJECTS).filter(f => f.language === language);
  }

  /**
   * Get fixtures by framework
   */
  static getByFramework(framework: string): ProjectFixture[] {
    return Object.values(this.PROJECTS).filter(f => f.framework === framework);
  }

  /**
   * Check if fixture exists and is accessible
   */
  static async exists(name: string): Promise<boolean> {
    try {
      const fixture = this.PROJECTS[name];
      if (!fixture) return false;
      
      await access(fixture.path);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Get expected output for a fixture
   */
  static async getExpectedOutput(type: string, name: string): Promise<any> {
    try {
      const outputPath = join(
        this.FIXTURES_BASE, 
        'expected-outputs', 
        `${name}-${type}.json`
      );
      
      const content = await readFile(outputPath, 'utf-8');
      return JSON.parse(content);
    } catch {
      return null;
    }
  }

  /**
   * Get golden file content (expected outputs for regression testing)
   */
  static async getGoldenFile(category: string, name: string): Promise<any> {
    try {
      const goldenPath = join(
        this.FIXTURES_BASE,
        'golden',
        category,
        `${name}.json`
      );
      
      const content = await readFile(goldenPath, 'utf-8');
      return JSON.parse(content);
    } catch {
      return null;
    }
  }

  /**
   * Get Dockerfile golden file
   */
  static async getGoldenDockerfile(name: string): Promise<string | null> {
    try {
      const dockerfilePath = join(
        this.FIXTURES_BASE,
        'golden',
        'dockerfiles',
        `${name}.Dockerfile`
      );
      
      return await readFile(dockerfilePath, 'utf-8');
    } catch {
      return null;
    }
  }

  /**
   * Get Kubernetes manifest fixtures
   */
  static getK8sManifestPath(scenario: string = 'default'): string {
    return join(this.FIXTURES_BASE, 'k8s', scenario);
  }

  /**
   * Create test data for vulnerable image scanning
   */
  static getVulnerableDockerfile(): string {
    return `FROM node:14.15.0
# Known vulnerable version with CVEs

WORKDIR /app
COPY package*.json ./

# Install known vulnerable packages
RUN npm install lodash@4.17.15 express@4.16.4 request@2.88.0

COPY . .
EXPOSE 3000

# Running as root (security issue)
USER root
CMD ["node", "app.js"]`;
  }

  /**
   * Create secure Dockerfile for comparison
   */
  static getSecureDockerfile(): string {
    return `FROM node:18-alpine

# Create non-root user
RUN addgroup -g 1001 -S nodejs && \\
    adduser -S nextjs -u 1001 -G nodejs

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies with clean install
RUN npm ci --only=production && \\
    npm cache clean --force

# Copy application code
COPY --chown=nextjs:nodejs . .

# Switch to non-root user
USER nextjs

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\
  CMD curl -f http://localhost:3000/health || exit 1

CMD ["node", "app.js"]`;
  }

  /**
   * Get test environment configurations
   */
  static getTestEnvironments() {
    return {
      development: {
        replicas: 1,
        resources: {
          requests: { cpu: '100m', memory: '128Mi' },
          limits: { cpu: '200m', memory: '256Mi' }
        },
        env: {
          NODE_ENV: 'development',
          LOG_LEVEL: 'debug'
        }
      },
      staging: {
        replicas: 2,
        resources: {
          requests: { cpu: '200m', memory: '256Mi' },
          limits: { cpu: '500m', memory: '512Mi' }
        },
        env: {
          NODE_ENV: 'staging',
          LOG_LEVEL: 'info'
        }
      },
      production: {
        replicas: 3,
        resources: {
          requests: { cpu: '500m', memory: '512Mi' },
          limits: { cpu: '1000m', memory: '1Gi' }
        },
        env: {
          NODE_ENV: 'production',
          LOG_LEVEL: 'warn'
        }
      }
    };
  }

  /**
   * Validate that all fixtures are accessible
   */
  static async validateFixtures(): Promise<{ valid: boolean; missing: string[] }> {
    const missing: string[] = [];
    
    for (const name of Object.keys(this.PROJECTS)) {
      const exists = await this.exists(name);
      if (!exists) {
        missing.push(name);
      }
    }

    return {
      valid: missing.length === 0,
      missing
    };
  }
}