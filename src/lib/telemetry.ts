/**
 * PKSP Telemetry and Metrics Collection
 * Comprehensive monitoring and observability for PKSP operations
 */

import { z } from 'zod';
import type { Logger } from 'pino';
// Performance tracking utilities

/**
 * Telemetry event types
 */
export const TelemetryEventType = z.enum([
  // PKSP Core Events
  'pksp.startup',
  'pksp.shutdown',
  'pksp.component.loaded',
  'pksp.component.error',

  // Prompt Events
  'prompt.render.start',
  'prompt.render.success',
  'prompt.render.error',
  'prompt.cache.hit',
  'prompt.cache.miss',
  'prompt.version.resolved',
  'prompt.inheritance.processed',
  'prompt.abtest.variant.selected',

  // Strategy Events
  'strategy.selection.start',
  'strategy.selection.completed',
  'strategy.optimization.applied',
  'strategy.cost.calculated',

  // Policy Events
  'policy.enforcement.start',
  'policy.enforcement.completed',
  'policy.violation.detected',
  'policy.constraint.applied',

  // Knowledge Events
  'knowledge.query.start',
  'knowledge.query.completed',
  'knowledge.cache.hit',
  'knowledge.cache.miss',

  // Performance Events
  'performance.target.violated',
  'performance.optimization.applied',
  'memory.usage.high',
  'cache.eviction',

  // Error Events
  'error.handled',
  'error.unhandled',
  'timeout.exceeded',
]);

export type TelemetryEvent = z.infer<typeof TelemetryEventType>;

/**
 * Telemetry data structure
 */
export interface TelemetryData {
  eventType: TelemetryEvent;
  timestamp: number;
  correlationId: string | undefined;
  userId: string | undefined;
  sessionId: string | undefined;
  tool: string | undefined;

  // Performance metrics
  duration: number | undefined;
  memoryUsageMB: number | undefined;
  cpuUsagePercent: number | undefined;

  // PKSP-specific data
  promptId: string | undefined;
  promptVersion: string | undefined;
  strategyId: string | undefined;
  policyIds: string[] | undefined;
  knowledgeQuery: string | undefined;
  cacheKey: string | undefined;

  // Results and outcomes
  success: boolean;
  errorMessage: string | undefined;
  errorCode: string | undefined;

  // Business metrics
  costUsd: number | undefined;
  tokensUsed: number | undefined;
  responseQuality: number | undefined; // 0-1 scale

  // Custom metadata
  metadata: Record<string, unknown> | undefined;
}

/**
 * Telemetry metrics aggregation
 */
export interface TelemetryMetrics {
  // Volume metrics
  totalEvents: number;
  eventsPerSecond: number;
  eventsByType: Record<TelemetryEvent, number>;

  // Performance metrics
  averageLatency: number;
  p95Latency: number;
  p99Latency: number;
  errorRate: number;

  // PKSP-specific metrics
  promptRenderRate: number;
  cacheHitRate: number;
  policyViolationRate: number;
  strategyOptimizationRate: number;

  // Cost metrics
  totalCostUsd: number;
  averageCostPerOperation: number;
  costEfficiencyScore: number;

  // Quality metrics
  averageResponseQuality: number;
  userSatisfactionScore: number;
}

/**
 * Telemetry collector and processor
 */
export class TelemetryCollector {
  private events: TelemetryData[] = [];
  private logger?: Logger;
  // Performance tracking handled internally
  private maxEvents = 10000; // Keep last 10k events in memory
  private metricsCache: { metrics: TelemetryMetrics; timestamp: number } | null = null;
  private metricsCacheTTL = 60000; // 1 minute

  constructor(logger?: Logger) {
    if (logger) {
      this.logger = logger.child({ component: 'TelemetryCollector' });
    }
  }

  /**
   * Record a telemetry event
   */
  recordEvent(data: Partial<Omit<TelemetryData, 'timestamp'>>): void {
    if (!data.eventType) {
      throw new Error('eventType is required for telemetry events');
    }
    const event: TelemetryData = {
      eventType: data.eventType,
      timestamp: Date.now(),
      correlationId: data.correlationId || undefined,
      userId: data.userId || undefined,
      sessionId: data.sessionId || undefined,
      tool: data.tool || undefined,
      duration: data.duration || undefined,
      memoryUsageMB: data.memoryUsageMB || undefined,
      cpuUsagePercent: data.cpuUsagePercent || undefined,
      promptId: data.promptId || undefined,
      promptVersion: data.promptVersion || undefined,
      strategyId: data.strategyId || undefined,
      policyIds: data.policyIds || undefined,
      knowledgeQuery: data.knowledgeQuery || undefined,
      cacheKey: data.cacheKey || undefined,
      success: data.success ?? false,
      errorMessage: data.errorMessage || undefined,
      errorCode: data.errorCode || undefined,
      costUsd: data.costUsd || undefined,
      tokensUsed: data.tokensUsed || undefined,
      responseQuality: data.responseQuality || undefined,
      metadata: data.metadata || undefined,
    };

    // Add to events array
    this.events.push(event);

    // Trim events if we exceed max
    if (this.events.length > this.maxEvents) {
      this.events.splice(0, this.events.length - this.maxEvents);
    }

    // Invalidate metrics cache
    this.metricsCache = null;

    // Log based on event type and success
    const logLevel = this.getLogLevel(event);
    this.logger?.[logLevel](
      {
        eventType: event.eventType,
        success: event.success,
        duration: event.duration,
        correlationId: event.correlationId,
        tool: event.tool,
        errorMessage: event.errorMessage,
        metadata: event.metadata,
      },
      'Telemetry event recorded',
    );

    // Check for critical issues
    this.checkCriticalIssues(event);
  }

  /**
   * Start tracking an operation
   */
  startOperation(eventType: TelemetryEvent, metadata?: Partial<TelemetryData>): string {
    const correlationId = this.generateCorrelationId();

    this.recordEvent({
      eventType,
      correlationId,
      success: true, // Start events are always successful
      ...metadata,
    });

    return correlationId;
  }

  /**
   * End tracking an operation
   */
  endOperation(
    correlationId: string,
    eventType: TelemetryEvent,
    result: { success: boolean; error?: string; metadata?: Record<string, unknown> },
    startTime?: number,
  ): void {
    const duration = startTime ? Date.now() - startTime : undefined;

    this.recordEvent({
      eventType,
      correlationId,
      success: result.success,
      errorMessage: result.error,
      duration,
      ...result.metadata,
    });
  }

  /**
   * Record prompt rendering telemetry
   */
  recordPromptRender(data: {
    promptId: string;
    promptVersion?: string;
    success: boolean;
    duration: number;
    cacheHit?: boolean;
    errorMessage?: string;
    correlationId?: string;
    metadata?: Record<string, unknown>;
  }): void {
    this.recordEvent({
      eventType: data.success ? 'prompt.render.success' : 'prompt.render.error',
      promptId: data.promptId,
      promptVersion: data.promptVersion,
      success: data.success,
      duration: data.duration,
      errorMessage: data.errorMessage,
      correlationId: data.correlationId,
      metadata: {
        cacheHit: data.cacheHit,
        ...data.metadata,
      },
    });

    // Also record cache event if applicable
    if (data.cacheHit !== undefined) {
      this.recordEvent({
        eventType: data.cacheHit ? 'prompt.cache.hit' : 'prompt.cache.miss',
        promptId: data.promptId,
        success: true,
        correlationId: data.correlationId,
      });
    }
  }

  /**
   * Record strategy selection telemetry
   */
  recordStrategySelection(data: {
    strategyId: string;
    tool: string;
    success: boolean;
    duration: number;
    costUsd?: number;
    errorMessage?: string;
    correlationId?: string;
    metadata?: Record<string, unknown>;
  }): void {
    this.recordEvent({
      eventType: 'strategy.selection.completed',
      strategyId: data.strategyId,
      tool: data.tool,
      success: data.success,
      duration: data.duration,
      costUsd: data.costUsd,
      errorMessage: data.errorMessage,
      correlationId: data.correlationId,
      metadata: data.metadata,
    });
  }

  /**
   * Record policy enforcement telemetry
   */
  recordPolicyEnforcement(data: {
    policyIds: string[];
    tool: string;
    success: boolean;
    duration: number;
    violationsDetected: number;
    errorMessage?: string;
    correlationId?: string;
  }): void {
    this.recordEvent({
      eventType: 'policy.enforcement.completed',
      policyIds: data.policyIds,
      tool: data.tool,
      success: data.success,
      duration: data.duration,
      errorMessage: data.errorMessage,
      correlationId: data.correlationId,
      metadata: {
        violationsDetected: data.violationsDetected,
      },
    });

    // Record violation events if any
    if (data.violationsDetected > 0) {
      this.recordEvent({
        eventType: 'policy.violation.detected',
        policyIds: data.policyIds,
        tool: data.tool,
        success: true,
        correlationId: data.correlationId,
        metadata: {
          violationCount: data.violationsDetected,
        },
      });
    }
  }

  /**
   * Record A/B test telemetry
   */
  recordABTest(data: {
    testId: string;
    variantId: string;
    promptId: string;
    tool: string;
    success: boolean;
    responseQuality?: number;
    correlationId?: string;
  }): void {
    this.recordEvent({
      eventType: 'prompt.abtest.variant.selected',
      promptId: data.promptId,
      tool: data.tool,
      success: data.success,
      responseQuality: data.responseQuality,
      correlationId: data.correlationId,
      metadata: {
        testId: data.testId,
        variantId: data.variantId,
      },
    });
  }

  /**
   * Get aggregated metrics
   */
  getMetrics(): TelemetryMetrics {
    // Return cached metrics if valid
    if (this.metricsCache && Date.now() - this.metricsCache.timestamp < this.metricsCacheTTL) {
      return this.metricsCache.metrics;
    }

    const now = Date.now();
    const oneHourAgo = now - 3600000; // 1 hour
    const recentEvents = this.events.filter((e) => e.timestamp > oneHourAgo);

    // Event volume metrics
    const totalEvents = recentEvents.length;
    const eventsPerSecond = totalEvents / 3600; // Events per second over last hour

    const eventsByType: Record<TelemetryEvent, number> = {} as any;
    for (const eventType of TelemetryEventType.options) {
      eventsByType[eventType] = recentEvents.filter((e) => e.eventType === eventType).length;
    }

    // Performance metrics
    const eventsWithDuration = recentEvents.filter((e) => e.duration !== undefined);
    const durations = eventsWithDuration.map((e) => e.duration as number).sort((a, b) => a - b);
    const averageLatency =
      durations.length > 0 ? durations.reduce((sum, d) => sum + d, 0) / durations.length : 0;
    const p95Index = Math.floor(durations.length * 0.95);
    const p99Index = Math.floor(durations.length * 0.99);
    const p95Latency = durations.length > 0 ? durations[p95Index] || 0 : 0;
    const p99Latency = durations.length > 0 ? durations[p99Index] || 0 : 0;

    const errorEvents = recentEvents.filter((e) => !e.success);
    const errorRate = totalEvents > 0 ? errorEvents.length / totalEvents : 0;

    // PKSP-specific metrics
    const promptRenderEvents = recentEvents.filter((e) => e.eventType.startsWith('prompt.render'));
    const promptRenderRate = promptRenderEvents.length / 3600;

    const cacheHitEvents = recentEvents.filter((e) => e.eventType === 'prompt.cache.hit');
    const cacheMissEvents = recentEvents.filter((e) => e.eventType === 'prompt.cache.miss');
    const totalCacheEvents = cacheHitEvents.length + cacheMissEvents.length;
    const cacheHitRate = totalCacheEvents > 0 ? cacheHitEvents.length / totalCacheEvents : 0;

    const policyViolationEvents = recentEvents.filter(
      (e) => e.eventType === 'policy.violation.detected',
    );
    const policyEnforcementEvents = recentEvents.filter(
      (e) => e.eventType === 'policy.enforcement.completed',
    );
    const policyViolationRate =
      policyEnforcementEvents.length > 0
        ? policyViolationEvents.length / policyEnforcementEvents.length
        : 0;

    const strategyOptimizationEvents = recentEvents.filter(
      (e) => e.eventType === 'strategy.optimization.applied',
    );
    const strategySelectionEvents = recentEvents.filter(
      (e) => e.eventType === 'strategy.selection.completed',
    );
    const strategyOptimizationRate =
      strategySelectionEvents.length > 0
        ? strategyOptimizationEvents.length / strategySelectionEvents.length
        : 0;

    // Cost metrics
    const eventsWithCost = recentEvents.filter((e) => e.costUsd !== undefined);
    const totalCostUsd = eventsWithCost.reduce((sum, e) => sum + (e.costUsd || 0), 0);
    const averageCostPerOperation =
      eventsWithCost.length > 0 ? totalCostUsd / eventsWithCost.length : 0;

    // Cost efficiency: operations per dollar (higher is better)
    const costEfficiencyScore = totalCostUsd > 0 ? totalEvents / totalCostUsd : 0;

    // Quality metrics
    const eventsWithQuality = recentEvents.filter((e) => e.responseQuality !== undefined);
    const averageResponseQuality =
      eventsWithQuality.length > 0
        ? eventsWithQuality.reduce((sum, e) => sum + (e.responseQuality || 0), 0) /
          eventsWithQuality.length
        : 0;

    // User satisfaction approximation (inverse of error rate)
    const userSatisfactionScore = Math.max(0, 1 - errorRate);

    const metrics: TelemetryMetrics = {
      totalEvents,
      eventsPerSecond,
      eventsByType,
      averageLatency,
      p95Latency,
      p99Latency,
      errorRate,
      promptRenderRate,
      cacheHitRate,
      policyViolationRate,
      strategyOptimizationRate,
      totalCostUsd,
      averageCostPerOperation,
      costEfficiencyScore,
      averageResponseQuality,
      userSatisfactionScore,
    };

    // Cache the metrics
    this.metricsCache = { metrics, timestamp: now };

    return metrics;
  }

  /**
   * Get recent events with filtering
   */
  getEvents(filter?: {
    eventType?: TelemetryEvent;
    success?: boolean;
    tool?: string;
    since?: number;
    limit?: number;
  }): TelemetryData[] {
    let filteredEvents = [...this.events];

    if (filter?.since !== undefined) {
      const sinceTimestamp = filter.since;
      filteredEvents = filteredEvents.filter((e) => e.timestamp >= sinceTimestamp);
    }

    if (filter?.eventType) {
      filteredEvents = filteredEvents.filter((e) => e.eventType === filter.eventType);
    }

    if (filter?.success !== undefined) {
      filteredEvents = filteredEvents.filter((e) => e.success === filter.success);
    }

    if (filter?.tool) {
      filteredEvents = filteredEvents.filter((e) => e.tool === filter.tool);
    }

    // Sort by timestamp descending (most recent first)
    filteredEvents.sort((a, b) => b.timestamp - a.timestamp);

    if (filter?.limit) {
      filteredEvents = filteredEvents.slice(0, filter.limit);
    }

    return filteredEvents;
  }

  /**
   * Export telemetry data
   */
  exportTelemetry(): {
    events: TelemetryData[];
    metrics: TelemetryMetrics;
    exportTime: string;
  } {
    return {
      events: [...this.events],
      metrics: this.getMetrics(),
      exportTime: new Date().toISOString(),
    };
  }

  /**
   * Clear telemetry data
   */
  clear(): void {
    this.events = [];
    this.metricsCache = null;
    this.logger?.info('Telemetry data cleared');
  }

  /**
   * Get health status based on telemetry
   */
  getHealthStatus(): {
    status: 'healthy' | 'warning' | 'critical';
    issues: string[];
    recommendations: string[];
  } {
    const metrics = this.getMetrics();
    const issues: string[] = [];
    const recommendations: string[] = [];

    // Check error rate
    if (metrics.errorRate > 0.1) {
      issues.push(`High error rate: ${(metrics.errorRate * 100).toFixed(1)}%`);
      recommendations.push('Investigate error patterns and implement fixes');
    } else if (metrics.errorRate > 0.05) {
      issues.push(`Elevated error rate: ${(metrics.errorRate * 100).toFixed(1)}%`);
      recommendations.push('Monitor error trends and consider preventive measures');
    }

    // Check latency
    if (metrics.p99Latency > 10000) {
      issues.push(`High P99 latency: ${metrics.p99Latency}ms`);
      recommendations.push('Optimize slow operations and consider caching');
    } else if (metrics.p99Latency > 5000) {
      issues.push(`Elevated P99 latency: ${metrics.p99Latency}ms`);
      recommendations.push('Monitor latency trends and optimize where possible');
    }

    // Check cache hit rate
    if (metrics.cacheHitRate < 0.5) {
      issues.push(`Low cache hit rate: ${(metrics.cacheHitRate * 100).toFixed(1)}%`);
      recommendations.push('Review cache configuration and TTL settings');
    }

    // Check policy violations
    if (metrics.policyViolationRate > 0.2) {
      issues.push(`High policy violation rate: ${(metrics.policyViolationRate * 100).toFixed(1)}%`);
      recommendations.push('Review policy configuration and enforcement logic');
    }

    // Determine overall status
    let status: 'healthy' | 'warning' | 'critical' = 'healthy';
    if (metrics.errorRate > 0.1 || metrics.p99Latency > 10000) {
      status = 'critical';
    } else if (issues.length > 0) {
      status = 'warning';
    }

    return { status, issues, recommendations };
  }

  // Private helper methods

  private getLogLevel(event: TelemetryData): 'debug' | 'info' | 'warn' | 'error' {
    if (!event.success) {
      return event.eventType.includes('error') ? 'error' : 'warn';
    }

    if (event.eventType.includes('violation') || event.eventType.includes('target.violated')) {
      return 'warn';
    }

    if (event.eventType.includes('start') || event.eventType.includes('cache')) {
      return 'debug';
    }

    return 'info';
  }

  private checkCriticalIssues(event: TelemetryData): void {
    // Check for performance violations
    if (event.eventType === 'performance.target.violated') {
      this.logger?.warn(
        {
          eventType: event.eventType,
          duration: event.duration,
          tool: event.tool,
        },
        'Performance target violation detected',
      );
    }

    // Check for high memory usage
    if (event.memoryUsageMB && event.memoryUsageMB > 100) {
      this.recordEvent({
        eventType: 'memory.usage.high',
        success: true,
        memoryUsageMB: event.memoryUsageMB,
        tool: event.tool,
        correlationId: event.correlationId,
      });
    }

    // Check for timeout exceeded
    if (event.errorMessage?.includes('timeout') || event.errorMessage?.includes('timeout')) {
      this.recordEvent({
        eventType: 'timeout.exceeded',
        success: false,
        errorMessage: event.errorMessage,
        tool: event.tool,
        correlationId: event.correlationId,
      });
    }
  }

  private generateCorrelationId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }
}

/**
 * Global telemetry collector instance
 */
let telemetryInstance: TelemetryCollector | null = null;

/**
 * Get or create telemetry collector instance
 */
export function getTelemetryCollector(logger?: Logger): TelemetryCollector {
  if (!telemetryInstance) {
    telemetryInstance = new TelemetryCollector(logger);
  }
  return telemetryInstance;
}

/**
 * Convenience functions for common telemetry operations
 */
export const telemetry = {
  /**
   * Record a simple event
   */
  record(eventType: TelemetryEvent, data: Partial<TelemetryData> = {}): void {
    getTelemetryCollector().recordEvent({
      eventType,
      success: true,
      ...data,
    });
  },

  /**
   * Start tracking an operation
   */
  start(eventType: TelemetryEvent, metadata?: Partial<TelemetryData>): string {
    return getTelemetryCollector().startOperation(eventType, metadata);
  },

  /**
   * End tracking an operation
   */
  end(
    correlationId: string,
    eventType: TelemetryEvent,
    result: { success: boolean; error?: string },
    startTime?: number,
  ): void {
    getTelemetryCollector().endOperation(correlationId, eventType, result, startTime);
  },

  /**
   * Get current metrics
   */
  getMetrics(): TelemetryMetrics {
    return getTelemetryCollector().getMetrics();
  },

  /**
   * Get health status
   */
  getHealth(): ReturnType<TelemetryCollector['getHealthStatus']> {
    return getTelemetryCollector().getHealthStatus();
  },
};
