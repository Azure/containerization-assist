import { E2ETestBase, E2ETestContext, E2ETestConfig } from '../../e2e/helpers/e2e-test-base';
import { Result, Success, Failure } from "../../src/types"
import { TestRepository } from '../../fixtures/types';

export interface PerformanceMetrics {
  executionTime: number;
  memoryUsage: {
    heapUsed: number;
    heapTotal: number;
    external: number;
    rss: number;
  };
  cpuUsage: {
    user: number;
    system: number;
  };
  operationsPerSecond?: number;
  throughput?: number;
  latency?: {
    min: number;
    max: number;
    avg: number;
    p95: number;
    p99: number;
  };
}

export interface PerformanceBenchmark {
  testName: string;
  iterations: number;
  metrics: PerformanceMetrics[];
  baseline?: PerformanceMetrics;
  threshold?: PerformanceThreshold;
  passed: boolean;
  regression?: boolean;
}

export interface PerformanceThreshold {
  maxExecutionTime: number;
  maxMemoryUsage: number;
  minOperationsPerSecond?: number;
  maxLatencyP95?: number;
  maxCpuUsage?: number;
}

export interface PerformanceTestConfig extends E2ETestConfig {
  iterations?: number;
  warmupRounds?: number;
  concurrency?: number;
  benchmarkBaseline?: boolean;
  thresholds?: PerformanceThreshold;
  enableProfiling?: boolean;
  collectGCMetrics?: boolean;
}

export class PerformanceTestBase extends E2ETestBase {
  private performanceConfig: PerformanceTestConfig;
  private benchmarks: Map<string, PerformanceBenchmark> = new Map();

  constructor(config: PerformanceTestConfig = {}) {
    super(config);
    this.performanceConfig = {
      iterations: 10,
      warmupRounds: 3,
      concurrency: 1,
      benchmarkBaseline: false,
      enableProfiling: false,
      collectGCMetrics: false,
      ...config
    };
  }

  async runPerformanceTest(
    testName: string,
    testFunction: () => Promise<any>,
    config?: Partial<PerformanceTestConfig>
  ): Promise<Result<PerformanceBenchmark>> {
    const testConfig = { ...this.performanceConfig, ...config };
    
    try {
      const metrics: PerformanceMetrics[] = [];
      
      // Warmup rounds
      for (let i = 0; i < testConfig.warmupRounds!; i++) {
        await testFunction();
        if (testConfig.collectGCMetrics && global.gc) {
          global.gc();
        }
      }

      // Performance test runs
      for (let i = 0; i < testConfig.iterations!; i++) {
        const startTime = process.hrtime.bigint();
        const startCpuUsage = process.cpuUsage();
        const startMemory = process.memoryUsage();

        await testFunction();

        const endTime = process.hrtime.bigint();
        const endCpuUsage = process.cpuUsage(startCpuUsage);
        const endMemory = process.memoryUsage();

        const executionTime = Number(endTime - startTime) / 1_000_000; // Convert to milliseconds

        metrics.push({
          executionTime,
          memoryUsage: {
            heapUsed: endMemory.heapUsed,
            heapTotal: endMemory.heapTotal,
            external: endMemory.external,
            rss: endMemory.rss
          },
          cpuUsage: {
            user: endCpuUsage.user / 1000, // Convert to milliseconds
            system: endCpuUsage.system / 1000
          }
        });

        // Optional GC between iterations
        if (testConfig.collectGCMetrics && global.gc) {
          global.gc();
        }
      }

      // Calculate derived metrics
      const executionTimes = metrics.map(m => m.executionTime);
      const totalTime = executionTimes.reduce((sum, time) => sum + time, 0);
      const avgExecutionTime = totalTime / executionTimes.length;
      
      // Calculate latency percentiles
      const sortedTimes = [...executionTimes].sort((a, b) => a - b);
      const latency = {
        min: sortedTimes[0],
        max: sortedTimes[sortedTimes.length - 1],
        avg: avgExecutionTime,
        p95: this.calculatePercentile(sortedTimes, 95),
        p99: this.calculatePercentile(sortedTimes, 99)
      };

      // Add latency to each metric
      metrics.forEach(metric => {
        metric.latency = latency;
        metric.operationsPerSecond = 1000 / metric.executionTime;
      });

      const benchmark: PerformanceBenchmark = {
        testName,
        iterations: testConfig.iterations!,
        metrics,
        threshold: testConfig.thresholds,
        passed: this.evaluatePerformance(metrics, testConfig.thresholds),
        regression: false // Would be determined by comparing with baseline
      };

      this.benchmarks.set(testName, benchmark);
      return Success(benchmark);

    } catch (error) {
      return Failure(`Performance test failed: ${error.message}`);
    }
  }

  async runConcurrentPerformanceTest(
    testName: string,
    testFunction: () => Promise<any>,
    concurrency: number = 5
  ): Promise<Result<PerformanceBenchmark>> {
    try {
      const startTime = process.hrtime.bigint();
      const startCpuUsage = process.cpuUsage();
      const startMemory = process.memoryUsage();

      // Create array of promises for concurrent execution
      const promises: Promise<any>[] = [];
      for (let i = 0; i < concurrency; i++) {
        promises.push(testFunction());
      }

      // Wait for all concurrent operations to complete
      await Promise.all(promises);

      const endTime = process.hrtime.bigint();
      const endCpuUsage = process.cpuUsage(startCpuUsage);
      const endMemory = process.memoryUsage();

      const executionTime = Number(endTime - startTime) / 1_000_000;
      const throughput = (concurrency * 1000) / executionTime; // operations per second

      const metrics: PerformanceMetrics[] = [{
        executionTime,
        memoryUsage: {
          heapUsed: endMemory.heapUsed,
          heapTotal: endMemory.heapTotal,
          external: endMemory.external,
          rss: endMemory.rss
        },
        cpuUsage: {
          user: endCpuUsage.user / 1000,
          system: endCpuUsage.system / 1000
        },
        throughput,
        operationsPerSecond: throughput
      }];

      const benchmark: PerformanceBenchmark = {
        testName: `${testName}_concurrent_${concurrency}`,
        iterations: 1,
        metrics,
        passed: true, // For concurrent tests, we mainly measure throughput
        regression: false
      };

      this.benchmarks.set(benchmark.testName, benchmark);
      return Success(benchmark);

    } catch (error) {
      return Failure(`Concurrent performance test failed: ${error.message}`);
    }
  }

  async runLoadTest(
    testName: string,
    testFunction: () => Promise<any>,
    duration: number = 30000, // 30 seconds
    targetRPS: number = 10 // requests per second
  ): Promise<Result<PerformanceBenchmark>> {
    try {
      const metrics: PerformanceMetrics[] = [];
      const startTime = Date.now();
      const interval = 1000 / targetRPS; // milliseconds between requests
      let operationCount = 0;

      while (Date.now() - startTime < duration) {
        const operationStart = process.hrtime.bigint();
        const startMemory = process.memoryUsage();

        await testFunction();

        const operationEnd = process.hrtime.bigint();
        const endMemory = process.memoryUsage();
        
        const executionTime = Number(operationEnd - operationStart) / 1_000_000;
        
        metrics.push({
          executionTime,
          memoryUsage: {
            heapUsed: endMemory.heapUsed,
            heapTotal: endMemory.heapTotal,
            external: endMemory.external,
            rss: endMemory.rss
          },
          cpuUsage: { user: 0, system: 0 }, // Not measured per operation in load test
          operationsPerSecond: 1000 / executionTime
        });

        operationCount++;

        // Wait for next interval (if operation was faster than target)
        const elapsed = Number(process.hrtime.bigint() - operationStart) / 1_000_000;
        if (elapsed < interval) {
          await new Promise(resolve => setTimeout(resolve, interval - elapsed));
        }
      }

      const totalDuration = Date.now() - startTime;
      const actualRPS = (operationCount * 1000) / totalDuration;

      const benchmark: PerformanceBenchmark = {
        testName: `${testName}_load_${targetRPS}rps`,
        iterations: operationCount,
        metrics,
        passed: actualRPS >= targetRPS * 0.9, // Allow 10% tolerance
        regression: false
      };

      benchmark.metrics.forEach(metric => {
        metric.throughput = actualRPS;
      });

      this.benchmarks.set(benchmark.testName, benchmark);
      return Success(benchmark);

    } catch (error) {
      return Failure(`Load test failed: ${error.message}`);
    }
  }

  getBenchmark(testName: string): PerformanceBenchmark | undefined {
    return this.benchmarks.get(testName);
  }

  getAllBenchmarks(): Map<string, PerformanceBenchmark> {
    return new Map(this.benchmarks);
  }

  generatePerformanceReport(): string {
    let report = '# Performance Test Report\n\n';
    report += `Generated: ${new Date().toISOString()}\n\n`;

    for (const [testName, benchmark] of this.benchmarks) {
      report += `## ${testName}\n\n`;
      report += `- **Status**: ${benchmark.passed ? '✅ PASS' : '❌ FAIL'}\n`;
      report += `- **Iterations**: ${benchmark.iterations}\n`;
      
      if (benchmark.metrics.length > 0) {
        const avgMetrics = this.calculateAverageMetrics(benchmark.metrics);
        report += `- **Avg Execution Time**: ${avgMetrics.executionTime.toFixed(2)}ms\n`;
        report += `- **Avg Memory (Heap Used)**: ${(avgMetrics.memoryUsage.heapUsed / 1024 / 1024).toFixed(2)}MB\n`;
        
        if (avgMetrics.operationsPerSecond) {
          report += `- **Operations/sec**: ${avgMetrics.operationsPerSecond.toFixed(2)}\n`;
        }
        
        if (avgMetrics.throughput) {
          report += `- **Throughput**: ${avgMetrics.throughput.toFixed(2)} ops/sec\n`;
        }
        
        if (avgMetrics.latency) {
          report += `- **Latency P95**: ${avgMetrics.latency.p95.toFixed(2)}ms\n`;
          report += `- **Latency P99**: ${avgMetrics.latency.p99.toFixed(2)}ms\n`;
        }
      }
      
      report += '\n';
    }

    return report;
  }

  private calculatePercentile(sortedValues: number[], percentile: number): number {
    const index = Math.ceil((percentile / 100) * sortedValues.length) - 1;
    return sortedValues[Math.max(0, Math.min(index, sortedValues.length - 1))];
  }

  private evaluatePerformance(metrics: PerformanceMetrics[], threshold?: PerformanceThreshold): boolean {
    if (!threshold) return true;

    const avgMetrics = this.calculateAverageMetrics(metrics);

    if (threshold.maxExecutionTime && avgMetrics.executionTime > threshold.maxExecutionTime) {
      return false;
    }

    if (threshold.maxMemoryUsage && avgMetrics.memoryUsage.heapUsed > threshold.maxMemoryUsage) {
      return false;
    }

    if (threshold.minOperationsPerSecond && avgMetrics.operationsPerSecond && 
        avgMetrics.operationsPerSecond < threshold.minOperationsPerSecond) {
      return false;
    }

    if (threshold.maxLatencyP95 && avgMetrics.latency && 
        avgMetrics.latency.p95 > threshold.maxLatencyP95) {
      return false;
    }

    return true;
  }

  private calculateAverageMetrics(metrics: PerformanceMetrics[]): PerformanceMetrics {
    const count = metrics.length;
    
    return {
      executionTime: metrics.reduce((sum, m) => sum + m.executionTime, 0) / count,
      memoryUsage: {
        heapUsed: metrics.reduce((sum, m) => sum + m.memoryUsage.heapUsed, 0) / count,
        heapTotal: metrics.reduce((sum, m) => sum + m.memoryUsage.heapTotal, 0) / count,
        external: metrics.reduce((sum, m) => sum + m.memoryUsage.external, 0) / count,
        rss: metrics.reduce((sum, m) => sum + m.memoryUsage.rss, 0) / count
      },
      cpuUsage: {
        user: metrics.reduce((sum, m) => sum + m.cpuUsage.user, 0) / count,
        system: metrics.reduce((sum, m) => sum + m.cpuUsage.system, 0) / count
      },
      operationsPerSecond: metrics[0].operationsPerSecond ? 
        metrics.reduce((sum, m) => sum + (m.operationsPerSecond || 0), 0) / count : undefined,
      throughput: metrics[0].throughput ?
        metrics.reduce((sum, m) => sum + (m.throughput || 0), 0) / count : undefined,
      latency: metrics[0].latency ? {
        min: Math.min(...metrics.map(m => m.latency?.min || Infinity)),
        max: Math.max(...metrics.map(m => m.latency?.max || 0)),
        avg: metrics.reduce((sum, m) => sum + (m.latency?.avg || 0), 0) / count,
        p95: metrics.reduce((sum, m) => sum + (m.latency?.p95 || 0), 0) / count,
        p99: metrics.reduce((sum, m) => sum + (m.latency?.p99 || 0), 0) / count
      } : undefined
    };
  }
}