/**
 * PKSP Performance Monitoring and Optimization
 * Profiles and optimizes PKSP operations for better performance
 */

import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';

/**
 * Performance metrics collection
 */
export interface PerformanceMetrics {
  operation: string;
  startTime: number;
  endTime: number;
  duration: number;
  memoryUsage: {
    before: NodeJS.MemoryUsage;
    after: NodeJS.MemoryUsage;
    delta: {
      rss: number;
      heapUsed: number;
      heapTotal: number;
      external: number;
    };
  };
  metadata: Record<string, unknown> | undefined;
}

/**
 * Performance targets and thresholds
 */
export interface PerformanceTargets {
  promptRenderingMs: number; // Target: <5ms (99th percentile)
  knowledgeQueryMs: number; // Target: <3ms (99th percentile)
  policyEnforcementMs: number; // Target: <1ms (99th percentile)
  startupTimeMs: number; // Target: <500ms total
  memoryUsageMB: number; // Target: <100MB for PKSP
}

const DEFAULT_TARGETS: PerformanceTargets = {
  promptRenderingMs: 5,
  knowledgeQueryMs: 3,
  policyEnforcementMs: 1,
  startupTimeMs: 500,
  memoryUsageMB: 100,
};

/**
 * Performance analyzer and optimizer
 */
export class PerformanceProfiler {
  private metrics: Map<string, PerformanceMetrics[]> = new Map();
  private targets: PerformanceTargets;
  private logger?: Logger;
  private activeOperations: Map<string, { startTime: number; memoryBefore: NodeJS.MemoryUsage }> =
    new Map();

  constructor(targets: Partial<PerformanceTargets> = {}, logger?: Logger) {
    this.targets = { ...DEFAULT_TARGETS, ...targets };
    if (logger) {
      this.logger = logger.child({ component: 'PerformanceProfiler' });
    }
  }

  /**
   * Start profiling an operation
   */
  startOperation(
    operationId: string,
    operationType: string,
    metadata?: Record<string, unknown>,
  ): string {
    const startTime = performance.now();
    const memoryBefore = process.memoryUsage();

    this.activeOperations.set(operationId, { startTime, memoryBefore });

    this.logger?.debug(
      {
        operationId,
        operationType,
        startTime,
        memoryBefore,
        metadata,
      },
      'Started profiling operation',
    );

    return operationId;
  }

  /**
   * End profiling an operation and record metrics
   */
  endOperation(
    operationId: string,
    operationType: string,
    metadata?: Record<string, unknown>,
  ): Result<PerformanceMetrics> {
    const active = this.activeOperations.get(operationId);
    if (!active) {
      return Failure(`Operation not found or already completed: ${operationId}`);
    }

    const endTime = performance.now();
    const memoryAfter = process.memoryUsage();
    const duration = endTime - active.startTime;

    const metrics: PerformanceMetrics = {
      operation: operationType,
      startTime: active.startTime,
      endTime,
      duration,
      memoryUsage: {
        before: active.memoryBefore,
        after: memoryAfter,
        delta: {
          rss: memoryAfter.rss - active.memoryBefore.rss,
          heapUsed: memoryAfter.heapUsed - active.memoryBefore.heapUsed,
          heapTotal: memoryAfter.heapTotal - active.memoryBefore.heapTotal,
          external: memoryAfter.external - active.memoryBefore.external,
        },
      },
      metadata,
    };

    // Store metrics
    if (!this.metrics.has(operationType)) {
      this.metrics.set(operationType, []);
    }
    const operationMetrics = this.metrics.get(operationType);
    if (!operationMetrics) {
      throw new Error(`Failed to initialize metrics for operation type: ${operationType}`);
    }
    operationMetrics.push(metrics);

    // Keep only last 1000 metrics per operation type
    if (operationMetrics.length > 1000) {
      operationMetrics.splice(0, operationMetrics.length - 1000);
    }

    // Clean up active operation
    this.activeOperations.delete(operationId);

    // Check if performance target is violated
    this.checkPerformanceTarget(operationType, duration);

    this.logger?.debug(
      {
        operationId,
        operationType,
        duration,
        memoryDelta: metrics.memoryUsage.delta,
        metadata,
      },
      'Completed profiling operation',
    );

    return Success(metrics);
  }

  /**
   * Profile a function execution
   */
  async profile<T>(
    operationType: string,
    fn: () => Promise<T> | T,
    metadata?: Record<string, unknown>,
  ): Promise<{ result: T; metrics: PerformanceMetrics }> {
    const operationId = `${operationType}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

    this.startOperation(operationId, operationType, metadata);

    try {
      const result = await fn();
      const metricsResult = this.endOperation(operationId, operationType, metadata);

      if (!metricsResult.ok) {
        throw new Error(`Failed to record metrics: ${metricsResult.error}`);
      }

      return { result, metrics: metricsResult.value };
    } catch (error) {
      // Ensure we clean up even on error
      this.activeOperations.delete(operationId);
      throw error;
    }
  }

  /**
   * Get performance statistics for an operation type
   */
  getStats(operationType: string): {
    count: number;
    mean: number;
    median: number;
    p95: number;
    p99: number;
    min: number;
    max: number;
    memoryStats: {
      avgHeapUsedMB: number;
      maxHeapUsedMB: number;
      avgRssMB: number;
      maxRssMB: number;
    };
  } | null {
    const operationMetrics = this.metrics.get(operationType);
    if (!operationMetrics || operationMetrics.length === 0) {
      return null;
    }

    const durations = operationMetrics.map((m) => m.duration).sort((a, b) => a - b);
    const heapUsages = operationMetrics.map((m) => m.memoryUsage.after.heapUsed / 1024 / 1024);
    const rssUsages = operationMetrics.map((m) => m.memoryUsage.after.rss / 1024 / 1024);

    const count = durations.length;
    const mean = durations.reduce((sum, d) => sum + d, 0) / count;
    const median = durations[Math.floor(count / 2)] || 0;
    const p95 = durations[Math.floor(count * 0.95)] || 0;
    const p99 = durations[Math.floor(count * 0.99)] || 0;
    const min = durations[0] || 0;
    const max = durations[count - 1] || 0;

    return {
      count,
      mean,
      median,
      p95,
      p99,
      min,
      max,
      memoryStats: {
        avgHeapUsedMB: heapUsages.reduce((sum, h) => sum + h, 0) / count,
        maxHeapUsedMB: Math.max(...heapUsages),
        avgRssMB: rssUsages.reduce((sum, r) => sum + r, 0) / count,
        maxRssMB: Math.max(...rssUsages),
      },
    };
  }

  /**
   * Get overall system performance report
   */
  getPerformanceReport(): {
    targets: PerformanceTargets;
    operations: Record<string, ReturnType<PerformanceProfiler['getStats']>>;
    violations: Array<{
      operation: string;
      target: number;
      actual: number;
      severity: 'warning' | 'critical';
    }>;
    recommendations: string[];
  } {
    const operations: Record<string, ReturnType<typeof this.getStats>> = {};
    const violations: Array<{
      operation: string;
      target: number;
      actual: number;
      severity: 'warning' | 'critical';
    }> = [];

    // Analyze each operation type
    for (const operationType of this.metrics.keys()) {
      const stats = this.getStats(operationType);
      operations[operationType] = stats;

      if (stats?.p99 !== undefined) {
        // Check against targets
        const target = this.getTargetForOperation(operationType);
        if (target && stats.p99 > target) {
          violations.push({
            operation: operationType,
            target,
            actual: stats.p99,
            severity: stats.p99 > target * 2 ? 'critical' : 'warning',
          });
        }
      }
    }

    // Generate recommendations
    const recommendations = this.generateRecommendations(operations, violations);

    return {
      targets: this.targets,
      operations,
      violations,
      recommendations,
    };
  }

  /**
   * Check if an operation violates performance targets
   */
  private checkPerformanceTarget(operationType: string, duration: number): void {
    const target = this.getTargetForOperation(operationType);
    if (target && duration > target) {
      const severity = duration > target * 2 ? 'error' : 'warn';
      this.logger?.[severity](
        {
          operationType,
          duration,
          target,
          violation: duration - target,
        },
        'Performance target violation',
      );
    }
  }

  /**
   * Get performance target for an operation type
   */
  private getTargetForOperation(operationType: string): number | null {
    const mapping: Record<string, keyof PerformanceTargets> = {
      'prompt-render': 'promptRenderingMs',
      'prompt-rendering': 'promptRenderingMs',
      'knowledge-query': 'knowledgeQueryMs',
      'policy-enforcement': 'policyEnforcementMs',
      startup: 'startupTimeMs',
    };

    const targetKey = mapping[operationType];
    return targetKey ? this.targets[targetKey] : null;
  }

  /**
   * Generate performance optimization recommendations
   */
  private generateRecommendations(
    operations: Record<string, ReturnType<typeof this.getStats>>,
    violations: Array<{
      operation: string;
      target: number;
      actual: number;
      severity: 'warning' | 'critical';
    }>,
  ): string[] {
    const recommendations: string[] = [];

    // Check for critical violations
    const criticalViolations = violations.filter((v) => v.severity === 'critical');
    if (criticalViolations.length > 0) {
      recommendations.push(
        `Critical performance issues found in: ${criticalViolations.map((v) => v.operation).join(', ')}. Immediate optimization required.`,
      );
    }

    // Memory recommendations
    for (const [operation, stats] of Object.entries(operations)) {
      if (stats && stats.memoryStats.maxHeapUsedMB > this.targets.memoryUsageMB) {
        recommendations.push(
          `High memory usage in ${operation} (${stats.memoryStats.maxHeapUsedMB.toFixed(1)}MB). Consider implementing caching limits or lazy loading.`,
        );
      }
    }

    // Prompt rendering optimization
    const promptStats = operations['prompt-render'] || operations['prompt-rendering'];
    if (promptStats && promptStats.p99 > this.targets.promptRenderingMs) {
      recommendations.push(
        'Prompt rendering is slow. Consider: 1) Template pre-compilation, 2) Parameter validation caching, 3) Simpler template logic.',
      );
    }

    // Knowledge query optimization
    const knowledgeStats = operations['knowledge-query'];
    if (knowledgeStats && knowledgeStats.p99 > this.targets.knowledgeQueryMs) {
      recommendations.push(
        'Knowledge queries are slow. Consider: 1) Indexing knowledge data, 2) Caching frequent queries, 3) Reducing knowledge pack size.',
      );
    }

    // General recommendations based on patterns
    if (recommendations.length === 0 && violations.length === 0) {
      recommendations.push(
        'Performance targets are being met. Continue monitoring for any regressions.',
      );
    }

    return recommendations;
  }

  /**
   * Reset all metrics
   */
  reset(): void {
    this.metrics.clear();
    this.activeOperations.clear();
    this.logger?.info('Performance metrics reset');
  }

  /**
   * Export metrics for external analysis
   */
  exportMetrics(): Record<string, PerformanceMetrics[]> {
    const exported: Record<string, PerformanceMetrics[]> = {};
    for (const [operation, metrics] of this.metrics.entries()) {
      exported[operation] = [...metrics]; // Clone to prevent external mutation
    }
    return exported;
  }
}

/**
 * Global performance profiler instance
 */
let profilerInstance: PerformanceProfiler | null = null;

/**
 * Get or create performance profiler instance
 */
export function getPerformanceProfiler(
  targets?: Partial<PerformanceTargets>,
  logger?: Logger,
): PerformanceProfiler {
  if (!profilerInstance) {
    profilerInstance = new PerformanceProfiler(targets, logger);
  }
  return profilerInstance;
}

/**
 * Convenience function to profile an operation
 */
export async function profileOperation<T>(
  operationType: string,
  fn: () => Promise<T> | T,
  metadata?: Record<string, unknown>,
  logger?: Logger,
): Promise<{ result: T; metrics: PerformanceMetrics }> {
  return getPerformanceProfiler({}, logger).profile(operationType, fn, metadata);
}

/**
 * Decorator for profiling class methods
 */
export function ProfileMethod(operationType?: string) {
  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    const originalMethod = descriptor.value;
    const operation = operationType || `${target.constructor.name}.${propertyKey}`;

    descriptor.value = async function (...args: any[]) {
      const profiler = getPerformanceProfiler();
      const { result } = await profiler.profile(operation, () => originalMethod.apply(this, args));
      return result;
    };

    return descriptor;
  };
}

/**
 * Performance-aware cache with TTL and memory limits
 */
export class PerformanceCache<K, V> {
  private cache = new Map<K, { value: V; timestamp: number; accessCount: number }>();
  private maxSize: number;
  private ttlMs: number;
  private profiler: PerformanceProfiler;

  constructor(maxSize = 1000, ttlMs = 60000, profiler?: PerformanceProfiler) {
    this.maxSize = maxSize;
    this.ttlMs = ttlMs;
    this.profiler = profiler || getPerformanceProfiler();
  }

  async get(key: K): Promise<V | undefined> {
    const { result } = await this.profiler.profile('cache-get', () => {
      const entry = this.cache.get(key);
      if (!entry) return undefined;

      // Check TTL
      if (Date.now() - entry.timestamp > this.ttlMs) {
        this.cache.delete(key);
        return undefined;
      }

      // Update access count for LRU
      entry.accessCount++;
      return entry.value;
    });

    return result;
  }

  async set(key: K, value: V): Promise<void> {
    await this.profiler.profile('cache-set', () => {
      // Evict if at capacity
      if (this.cache.size >= this.maxSize) {
        this.evictLRU();
      }

      this.cache.set(key, {
        value,
        timestamp: Date.now(),
        accessCount: 1,
      });
    });
  }

  private evictLRU(): void {
    let lruKey: K | undefined;
    let lruAccessCount = Infinity;

    for (const [key, entry] of this.cache.entries()) {
      if (entry.accessCount < lruAccessCount) {
        lruAccessCount = entry.accessCount;
        lruKey = key;
      }
    }

    if (lruKey !== undefined) {
      this.cache.delete(lruKey);
    }
  }

  clear(): void {
    this.cache.clear();
  }

  size(): number {
    return this.cache.size;
  }
}
