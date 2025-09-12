/**
 * Correlation Context for distributed tracing
 *
 * Provides correlation IDs and context propagation across async operations
 */

import { randomUUID } from 'crypto';
import { AsyncLocalStorage } from 'async_hooks';
import type { Logger } from 'pino';

export interface ContextData {
  correlationId: string;
  workflowId: string;
  spanId: string;
  parentSpanId?: string;
  startTime: number;
  metadata?: Record<string, unknown>;
  toolName?: string;
  sessionId?: string;
}

/**
 * Correlation context using AsyncLocalStorage for automatic propagation
 */
export class CorrelationContext {
  private static storage = new AsyncLocalStorage<ContextData>();

  /**
   * Run a function with correlation context
   */
  static run<T>(data: Partial<ContextData>, fn: () => T): T {
    const current = this.getCurrent();

    const merged: ContextData = {
      correlationId: data.correlationId || generateId(),
      workflowId: data.workflowId || current?.workflowId || generateId(),
      spanId: data.spanId || generateId(),
      startTime: data.startTime || Date.now(),
      metadata: { ...current?.metadata, ...data.metadata },
    };

    // Only set optional properties if they have values
    const parentSpanId = data.parentSpanId ?? current?.spanId;
    if (parentSpanId) merged.parentSpanId = parentSpanId;

    const toolName = data.toolName || current?.toolName;
    if (toolName) merged.toolName = toolName;

    const sessionId = data.sessionId || current?.sessionId;
    if (sessionId) merged.sessionId = sessionId;

    return this.storage.run(merged, fn);
  }

  /**
   * Run async function with correlation context
   */
  static async runAsync<T>(data: Partial<ContextData>, fn: () => Promise<T>): Promise<T> {
    const current = this.getCurrent();

    const merged: ContextData = {
      correlationId: data.correlationId || generateId(),
      workflowId: data.workflowId || current?.workflowId || generateId(),
      spanId: data.spanId || generateId(),
      startTime: data.startTime || Date.now(),
      metadata: { ...current?.metadata, ...data.metadata },
    };

    // Only set optional properties if they have values
    const parentSpanId = data.parentSpanId ?? current?.spanId;
    if (parentSpanId) merged.parentSpanId = parentSpanId;

    const toolName = data.toolName || current?.toolName;
    if (toolName) merged.toolName = toolName;

    const sessionId = data.sessionId || current?.sessionId;
    if (sessionId) merged.sessionId = sessionId;

    return this.storage.run(merged, fn);
  }

  /**
   * Get current context
   */
  static getCurrent(): ContextData | undefined {
    return this.storage.getStore();
  }

  /**
   * Create a child context (new span)
   */
  static createChild(metadata?: Record<string, unknown>): ContextData {
    const current = this.getCurrent();

    if (!current) {
      // No parent context, create new root
      return {
        correlationId: generateId(),
        workflowId: generateId(),
        spanId: generateId(),
        startTime: Date.now(),
        metadata: metadata || {},
      };
    }

    return {
      ...current,
      spanId: generateId(),
      parentSpanId: current.spanId,
      startTime: Date.now(),
      metadata: { ...current.metadata, ...metadata },
    };
  }

  /**
   * Extract context for external propagation (e.g., HTTP headers)
   */
  static extract(): Record<string, string> {
    const current = this.getCurrent();

    if (!current) {
      return {};
    }

    return {
      'x-correlation-id': current.correlationId,
      'x-workflow-id': current.workflowId,
      'x-span-id': current.spanId,
      'x-parent-span-id': current.parentSpanId || '',
      'x-session-id': current.sessionId || '',
    };
  }

  /**
   * Inject context from external source (e.g., HTTP headers)
   */
  static inject(headers: Record<string, string | undefined>): Partial<ContextData> {
    const result: Partial<ContextData> = {};

    if (headers['x-correlation-id']) result.correlationId = headers['x-correlation-id'];
    if (headers['x-workflow-id']) result.workflowId = headers['x-workflow-id'];
    if (headers['x-span-id']) result.spanId = headers['x-span-id'];
    if (headers['x-parent-span-id']) result.parentSpanId = headers['x-parent-span-id'];
    if (headers['x-session-id']) result.sessionId = headers['x-session-id'];

    return result;
  }

  /**
   * Calculate duration from span start
   */
  static getDuration(): number {
    const current = this.getCurrent();
    return current ? Date.now() - current.startTime : 0;
  }
}

/**
 * Generate unique ID
 */
export function generateId(): string {
  return randomUUID().substring(0, 8);
}

/**
 * Create a logger with correlation context
 */
export function createContextLogger(baseLogger: Logger): Logger {
  const context = CorrelationContext.getCurrent();

  if (!context) {
    return baseLogger;
  }

  return baseLogger.child({
    correlationId: context.correlationId,
    workflowId: context.workflowId,
    spanId: context.spanId,
    parentSpanId: context.parentSpanId,
    sessionId: context.sessionId,
    toolName: context.toolName,
  });
}

/**
 * Middleware for MCP tools to add correlation context
 */
export function correlationMiddleware<T extends (...args: any[]) => any>(
  handler: T,
  toolName?: string,
): T {
  return (async (...args: Parameters<T>) => {
    // Extract context from args if available
    const [params, context] = args;

    const correlationData: Partial<ContextData> = {
      correlationId: context?.correlationId,
      workflowId: context?.workflowId,
      sessionId: params?.sessionId || context?.sessionId,
      toolName: toolName || context?.toolName,
      metadata: {
        tool: toolName,
        params: Object.keys(params || {}),
      },
    };

    return CorrelationContext.runAsync(correlationData, async () => {
      return handler(...args);
    });
  }) as T;
}

/**
 * Decorator for adding correlation to class methods
 */
export function withCorrelation(toolName?: string) {
  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    const originalMethod = descriptor.value;

    descriptor.value = async function (...args: any[]) {
      const correlationData: Partial<ContextData> = {
        toolName: toolName || propertyKey,
        metadata: {
          class: target.constructor.name,
          method: propertyKey,
        },
      };

      return CorrelationContext.runAsync(correlationData, async () => {
        return originalMethod.apply(this, args);
      });
    };

    return descriptor;
  };
}

/**
 * Format correlation for logging
 */
export function formatCorrelation(context?: ContextData): Record<string, unknown> {
  if (!context) {
    context = CorrelationContext.getCurrent();
  }

  if (!context) {
    return {};
  }

  return {
    correlationId: context.correlationId,
    workflowId: context.workflowId,
    spanId: context.spanId,
    parentSpanId: context.parentSpanId,
    duration: Date.now() - context.startTime,
    tool: context.toolName,
    session: context.sessionId,
  };
}
