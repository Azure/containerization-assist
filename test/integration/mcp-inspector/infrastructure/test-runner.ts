/**
 * MCP Inspector Test Runner Framework
 * MCP Inspector Testing Infrastructure
 */

import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import type { TestResult, PerformanceMetrics } from '../../../../src/types/result-types.js';
import { extractErrorMessage, formatErrorMessage } from '../../../../src/lib/error-utils.js';

export interface TestCase {
  name: string;
  category: TestCategory;
  description?: string;
  setup?: () => Promise<void>;
  execute: () => Promise<TestInfrastructureResult>;
  cleanup?: () => Promise<void>;
  timeout?: number;
  tags?: string[];
}

// Using TestResult and PerformanceMetrics from result types
// Additional properties specific to test infrastructure
export interface TestInfrastructurePerformanceMetrics extends PerformanceMetrics {
  resourceSize?: number;
  operationCount?: number;
}

export interface TestInfrastructureResult extends TestResult {
  performance?: TestInfrastructurePerformanceMetrics;
}

export type TestCategory = 
  | 'tool-validation'
  | 'resource-management' 
  | 'sampling-validation'
  | 'integration-flows'
  | 'load-testing'
  | 'performance-benchmarks'
  | 'orchestrator'
  | 'remediation'
  | 'edge-cases';

export interface TestFilter {
  categories?: TestCategory[];
  tags?: string[];
  namePattern?: RegExp;
}

export interface TestSuiteResults {
  passed: number;
  failed: number;
  skipped: number;
  totalDuration: number;
  results: Array<TestInfrastructureResult & { testName: string }>;
  performance: {
    avgResponseTime: number;
    maxMemoryUsage: number;
    totalOperations: number;
  };
}

export class MCPTestRunner {
  private testCases: Map<string, TestCase> = new Map();
  private client: Client | null = null;
  private transport: StdioClientTransport | null = null;

  constructor(
    private serverScript: string = './scripts/mcp-start-mock.sh',
    private options: { timeout: number } = { timeout: 30000 }
  ) {}

  /**
   * Initialize MCP client connection
   */
  async initialize(): Promise<void> {
    this.transport = new StdioClientTransport({
      command: this.serverScript,
      args: ['start']
    });

    this.client = new Client({
      name: 'mcp-test-runner',
      version: '1.0.0'
    }, {
      capabilities: {}
    });

    await this.client.connect(this.transport);
  }

  /**
   * Clean up MCP client connection
   */
  async cleanup(): Promise<void> {
    if (this.client) {
      await this.client.close();
      this.client = null;
    }
    if (this.transport) {
      await this.transport.close();
      this.transport = null;
    }
  }

  /**
   * Register a test case
   */
  register(testCase: TestCase): void {
    if (this.testCases.has(testCase.name)) {
      throw new Error(`Test case '${testCase.name}' is already registered`);
    }
    this.testCases.set(testCase.name, testCase);
  }

  /**
   * Run all tests or filtered subset
   */
  async run(filter?: TestFilter): Promise<TestSuiteResults> {
    if (!this.client) {
      throw new Error('Test runner not initialized. Call initialize() first.');
    }

    const filteredTests = this.filterTests(filter);
    const results: Array<TestInfrastructureResult & { testName: string }> = [];
    let passed = 0;
    let failed = 0;
    let skipped = 0;
    const startTime = performance.now();

    for (const [testName, testCase] of filteredTests) {
      console.log(`Running test: ${testName}`);
      
      try {
        const result = await this.runSingleTest(testCase);
        results.push({ ...result, testName });
        
        if (result.success) {
          passed++;
          console.log(`✅ ${testName} - ${result.duration}ms`);
        } else {
          failed++;
          console.log(`❌ ${testName} - ${result.message}`);
        }
      } catch (error) {
        failed++;
        const errorMessage = extractErrorMessage(error);
        results.push({
          testName,
          success: false,
          duration: 0,
          message: formatErrorMessage('Test execution failed', error)
        });
        console.log(`💥 ${testName} - ${errorMessage}`);
      }
    }

    const totalDuration = performance.now() - startTime;
    
    return {
      passed,
      failed,
      skipped,
      totalDuration,
      results,
      performance: this.calculatePerformanceStats(results)
    };
  }

  /**
   * Run tests in parallel (for load testing)
   */
  async runParallel(testNames: string[], concurrency = 5): Promise<TestSuiteResults> {
    if (!this.client) {
      throw new Error('Test runner not initialized. Call initialize() first.');
    }

    const tests = testNames.map(name => this.testCases.get(name)).filter(Boolean) as TestCase[];
    const results: Array<TestInfrastructureResult & { testName: string }> = [];
    
    // Run tests in batches
    for (let i = 0; i < tests.length; i += concurrency) {
      const batch = tests.slice(i, i + concurrency);
      const batchPromises = batch.map(async (test) => {
        const result = await this.runSingleTest(test);
        return { ...result, testName: test.name };
      });

      const batchResults = await Promise.all(batchPromises);
      results.push(...batchResults);
    }

    const passed = results.filter(r => r.success).length;
    const failed = results.filter(r => !r.success).length;
    const totalDuration = results.reduce((sum, r) => sum + r.duration, 0);

    return {
      passed,
      failed,
      skipped: 0,
      totalDuration,
      results,
      performance: this.calculatePerformanceStats(results)
    };
  }

  /**
   * Get the MCP client for direct use in tests
   */
  getClient(): Client {
    if (!this.client) {
      throw new Error('Test runner not initialized. Call initialize() first.');
    }
    return this.client;
  }

  private async runSingleTest(testCase: TestCase): Promise<TestInfrastructureResult> {
    const startTime = performance.now();
    const initialMemory = process.memoryUsage().heapUsed;

    try {
      // Setup
      if (testCase.setup) {
        await testCase.setup();
      }

      // Execute with timeout
      const result = await Promise.race([
        testCase.execute(),
        this.timeoutPromise(testCase.timeout || this.options.timeout)
      ]);

      const endTime = performance.now();
      const finalMemory = process.memoryUsage().heapUsed;

      // Cleanup
      if (testCase.cleanup) {
        await testCase.cleanup();
      }

      return {
        ...result,
        duration: endTime - startTime,
        performance: {
          responseTime: result.performance?.responseTime || (endTime - startTime),
          memoryUsage: finalMemory - initialMemory,
          ...(result.performance?.resourceSize !== undefined && { resourceSize: result.performance.resourceSize }),
          ...(result.performance?.operationCount !== undefined && { operationCount: result.performance.operationCount })
        }
      };
    } catch (error) {
      const endTime = performance.now();
      return {
        success: false,
        duration: endTime - startTime,
        message: extractErrorMessage(error)
      };
    }
  }

  private filterTests(filter?: TestFilter): Map<string, TestCase> {
    if (!filter) {
      return this.testCases;
    }

    const filtered = new Map<string, TestCase>();
    
    for (const [name, testCase] of this.testCases) {
      let include = true;

      if (filter.categories && !filter.categories.includes(testCase.category)) {
        include = false;
      }

      if (filter.tags && testCase.tags) {
        const hasMatchingTag = filter.tags.some(tag => testCase.tags?.includes(tag));
        if (!hasMatchingTag) {
          include = false;
        }
      }

      if (filter.namePattern && !filter.namePattern.test(name)) {
        include = false;
      }

      if (include) {
        filtered.set(name, testCase);
      }
    }

    return filtered;
  }

  private calculatePerformanceStats(results: Array<TestInfrastructureResult & { testName: string }>) {
    const responseTimes = results.map(r => r.performance?.responseTime || r.duration);
    const memoryUsages = results.map(r => r.performance?.memoryUsage || 0);
    const operations = results.reduce((sum, r) => sum + (r.performance?.operationCount || 1), 0);

    return {
      avgResponseTime: responseTimes.reduce((sum, time) => sum + time, 0) / responseTimes.length,
      maxMemoryUsage: Math.max(...memoryUsages),
      totalOperations: operations
    };
  }

  private timeoutPromise(timeout: number): Promise<TestInfrastructureResult> {
    return new Promise((_, reject) => {
      setTimeout(() => {
        reject(new Error(`Test timed out after ${timeout}ms`));
      }, timeout);
    });
  }
}