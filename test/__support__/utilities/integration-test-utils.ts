/**
 * Integration Test Utilities
 * Standardized utilities for integration test setup and execution
 */

import type { Logger } from 'pino';
import { join } from 'path';
import { promises as fs } from 'fs';
import tmp from 'tmp';
import type { DirResult } from 'tmp';
import { DetectionOptions, EnvironmentCapabilities, detectEnvironment } from './environment-detector';

export interface IntegrationTestContext {
  capabilities: EnvironmentCapabilities;
  tempDirs: string[];
  cleanupTasks: (() => Promise<void>)[];
}

export interface ConditionalTestOptions {
  requirements: Array<keyof Omit<EnvironmentCapabilities, 'platform'>>;
  skipMessage?: string;
  timeout?: number;
  detectionOptions?: DetectionOptions;
}

/**
 * Enhanced logger for integration tests
 */
export function createTestLogger(prefix: string = 'integration-test'): Logger {
  const mockLogger = {
    child: jest.fn().mockReturnThis(),
    info: jest.fn(),
    debug: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn()
  } as unknown as Logger;

  // In development mode, forward to console for debugging
  if (process.env.NODE_ENV !== 'test') {
    const originalInfo = mockLogger.info as jest.MockedFunction<any>;
    const originalError = mockLogger.error as jest.MockedFunction<any>;
    const originalWarn = mockLogger.warn as jest.MockedFunction<any>;

    originalInfo.mockImplementation((...args) => {
      console.log(`[${prefix}]`, ...args);
    });

    originalError.mockImplementation((...args) => {
      console.error(`[${prefix}]`, ...args);
    });

    originalWarn.mockImplementation((...args) => {
      console.warn(`[${prefix}]`, ...args);
    });
  }

  return mockLogger;
}

/**
 * Cleanup test resources
 * Now uses tmp module for automatic cleanup on process exit
 */
export class IntegrationTestCleanup {
  private tempDirs: DirResult[] = [];
  private cleanupTasks: (() => Promise<void>)[] = [];

  /**
   * Create and track a temporary directory
   */
  createTempDir(prefix: string): string {
    const dir = tmp.dirSync({
      prefix: `${prefix}-`,
      unsafeCleanup: true,
      keep: false,
    });
    this.tempDirs.push(dir);
    return dir.name;
  }

  addCleanupTask(task: () => Promise<void>): void {
    this.cleanupTasks.push(task);
  }

  async cleanup(): Promise<void> {
    // Run custom cleanup tasks first
    for (const task of this.cleanupTasks) {
      try {
        await task();
      } catch (error) {
        console.warn('Cleanup task failed:', error);
      }
    }

    // Clean temp directories using tmp module
    for (const dir of this.tempDirs) {
      try {
        dir.removeCallback();
      } catch (error) {
        console.warn(`Failed to cleanup directory ${dir.name}:`, error);
      }
    }

    // Reset arrays
    this.tempDirs = [];
    this.cleanupTasks = [];
  }
}

/**
 * Enhanced conditional describe that handles environment detection
 */
export function describeWithEnvironment(
  testName: string, 
  options: ConditionalTestOptions,
  testFn: (context: IntegrationTestContext) => void
): void {
  const { requirements, skipMessage, timeout = 5000, detectionOptions } = options;

  describe(testName, () => {
    let testContext: IntegrationTestContext;
    let cleanup: IntegrationTestCleanup;

    beforeAll(async () => {
      // Detect environment capabilities
      const capabilities = await detectEnvironment({ 
        timeout,
        ...detectionOptions 
      });

      // Check if requirements are met
      const missingRequirements = requirements.filter(req => !capabilities[req].available);
      
      if (missingRequirements.length > 0) {
        const message = skipMessage || `Missing required services: ${missingRequirements.join(', ')}`;
        console.log(`⏭️ Skipping ${testName}: ${message}`);
        
        // Log detailed information about missing services
        for (const req of missingRequirements) {
          console.log(`   ${req}: ${capabilities[req].error}`);
        }
        
        // Mark all tests in this suite as pending
        describe('Environment Requirements Check', () => {
          test.skip('required services not available', () => {
            // This will show up as skipped in test output
          });
        });
        return;
      }

      // Initialize test context
      cleanup = new IntegrationTestCleanup();
      testContext = {
        capabilities,
        tempDirs: [],
        cleanupTasks: []
      };

      console.log(`✅ ${testName}: All requirements met`);
      for (const req of requirements) {
        const service = capabilities[req];
        if (service.available) {
          const version = 'version' in service ? service.version : '';
          console.log(`   ${req}: Available ${version ? `(${version})` : ''}`);
        }
      }
    }, timeout * 2); // Give extra time for environment detection

    afterAll(async () => {
      if (cleanup) {
        await cleanup.cleanup();
      }
    });

    // Only run tests if environment is suitable
    describe('Tests', () => {
      beforeEach(() => {
        if (!testContext) {
          pending('Environment requirements not met');
        }
      });

      // Run the actual test function
      testFn({
        get capabilities() {
          if (!testContext) {
            throw new Error('Test context not initialized - environment requirements not met');
          }
          return testContext.capabilities;
        },
        get tempDirs() {
          return testContext ? testContext.tempDirs : [];
        },
        get cleanupTasks() {
          return testContext ? testContext.cleanupTasks : [];
        }
      });
    });
  });
}

/**
 * Utility to create test files in a directory
 */
export async function createTestFiles(contextDir: string, files: Record<string, string>): Promise<void> {
  for (const [filename, content] of Object.entries(files)) {
    const filePath = join(contextDir, filename);
    
    // Create directory if needed
    const dirPath = join(filePath, '..');
    await fs.mkdir(dirPath, { recursive: true });
    
    await fs.writeFile(filePath, content.trim());
  }
}

/**
 * Standard Docker test files
 */
export function getStandardDockerFiles(): Record<string, string> {
  return {
    'Dockerfile': `
FROM node:18-alpine
LABEL maintainer="test@example.com"
LABEL version="1.0.0"
WORKDIR /app
COPY package.json ./
RUN echo '{"name": "test-app", "version": "1.0.0"}' > package.json || npm init -y
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]
`,
    'package.json': `
{
  "name": "integration-test-app",
  "version": "1.0.0",
  "description": "Test application for Docker integration tests",
  "main": "index.js",
  "dependencies": {}
}
`,
    'index.js': `
console.log('Hello from Docker integration test!');
console.log('Timestamp:', new Date().toISOString());
process.exit(0);
`,
    '.dockerignore': `
node_modules
npm-debug.log
.git
.gitignore
README.md
.env
.nyc_output
coverage
.nyc_output
`
  };
}

/**
 * Generate unique test identifiers to avoid conflicts
 */
export function generateTestId(): string {
  return `test-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * Wait for a condition to be true with timeout
 */
export async function waitFor(
  condition: () => Promise<boolean> | boolean,
  options: {
    timeout?: number;
    interval?: number;
    errorMessage?: string;
  } = {}
): Promise<void> {
  const { timeout = 30000, interval = 1000, errorMessage = 'Condition not met within timeout' } = options;
  
  const start = Date.now();
  
  while (Date.now() - start < timeout) {
    try {
      const result = await condition();
      if (result) {
        return;
      }
    } catch (error) {
      // Continue waiting even if condition throws
    }
    
    await new Promise(resolve => setTimeout(resolve, interval));
  }
  
  throw new Error(`${errorMessage} (waited ${timeout}ms)`);
}

/**
 * Retry an operation with exponential backoff
 */
export async function retryWithBackoff<T>(
  operation: () => Promise<T>,
  options: {
    maxAttempts?: number;
    baseDelay?: number;
    maxDelay?: number;
    factor?: number;
  } = {}
): Promise<T> {
  const { 
    maxAttempts = 3, 
    baseDelay = 1000, 
    maxDelay = 10000, 
    factor = 2 
  } = options;

  let lastError: Error;
  
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      return await operation();
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
      
      if (attempt === maxAttempts) {
        throw lastError;
      }
      
      const delay = Math.min(baseDelay * Math.pow(factor, attempt - 1), maxDelay);
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }
  
  throw lastError!;
}