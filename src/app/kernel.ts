/**
 * Application Kernel
 * Unified execution path for all tool invocations
 *
 * Architecture:
 * - SimpleRouter: Handles single tool execution without dependencies
 * - Kernel: Handles complex workflows, dependencies, and policy enforcement
 *
 * Routing Decision:
 * - Simple tools (no dependencies, no complex policies) → SimpleRouter
 * - Complex tools (with dependencies or policies) → Kernel orchestration
 *
 * This separation improves:
 * - Performance: Simple tools bypass unnecessary complexity
 * - Maintainability: Clear separation of concerns
 * - Testability: Isolated components are easier to test
 */

import { z } from 'zod';
import { type Result, Success, Failure } from '@/types/index';
import { createLogger } from '@/lib/logger';
import { loadPolicy } from '@/config/policy-io';
import { applyPolicy } from '@/config/policy-eval';
import type { Policy } from '@/config/policy-schemas';
import { runTool, canExecuteSimply } from '@/mcp/standalone-executor';
import { SessionManager as SimpleSessionManager } from '@/lib/session-manager';
import type {
  Kernel,
  KernelConfig,
  KernelFactoryOptions,
  ExecuteRequest,
  ExecutionPlan,
  RegisteredTool,
  ToolContext,
  SessionState,
  SessionManager,
  TelemetrySystem,
  TelemetryEvent,
  HealthStatus,
  AggregatedMetric,
  Logger,
  ProgressReporter,
} from './types';

// ============================================================================
// Default Implementations
// ============================================================================

/**
 * In-memory session manager using our SimpleSessionManager
 */
class InMemorySessionManager implements SessionManager {
  private manager = new SimpleSessionManager();

  async get(sessionId: string): Promise<Result<SessionState>> {
    if (!this.manager.has(sessionId)) {
      return Failure(`Session not found: ${sessionId}`);
    }

    // Get session data from simple manager
    const data = this.manager.get<Record<string, unknown>>(sessionId, 'data') || {};
    const completedSteps = this.manager.get<string[]>(sessionId, 'completed_steps') || [];
    const created = this.manager.get<Date>(sessionId, 'created') || new Date();
    const updated = this.manager.get<Date>(sessionId, 'updated') || new Date();
    const metadata = this.manager.get<Record<string, unknown>>(sessionId, 'metadata') || {};

    return Success({
      sessionId,
      created,
      updated,
      completed_steps: completedSteps,
      data,
      metadata,
    });
  }

  async create(): Promise<Result<SessionState>> {
    const sessionId = this.manager.ensureSession();
    const now = new Date();

    const session: SessionState = {
      sessionId,
      created: now,
      updated: now,
      completed_steps: [],
      data: {},
      metadata: {},
    };

    // Store session data
    this.manager.set(sessionId, 'created', now);
    this.manager.set(sessionId, 'updated', now);
    this.manager.set(sessionId, 'completed_steps', []);
    this.manager.set(sessionId, 'data', {});
    this.manager.set(sessionId, 'metadata', {});

    return Success(session);
  }

  async update(sessionId: string, updates: Partial<SessionState>): Promise<Result<void>> {
    if (!this.manager.has(sessionId)) {
      return Failure(`Session not found: ${sessionId}`);
    }

    // Update individual fields
    if (updates.data !== undefined) {
      const currentData = this.manager.get<Record<string, unknown>>(sessionId, 'data') || {};
      this.manager.set(sessionId, 'data', { ...currentData, ...updates.data });
    }

    if (updates.completed_steps !== undefined) {
      this.manager.set(sessionId, 'completed_steps', updates.completed_steps);
    }

    if (updates.metadata !== undefined) {
      const currentMetadata =
        this.manager.get<Record<string, unknown>>(sessionId, 'metadata') || {};
      this.manager.set(sessionId, 'metadata', { ...currentMetadata, ...updates.metadata });
    }

    // Always update the updated timestamp
    this.manager.set(sessionId, 'updated', new Date());

    return Success(undefined);
  }

  async delete(sessionId: string): Promise<Result<void>> {
    if (!this.manager.has(sessionId)) {
      return Failure(`Session not found: ${sessionId}`);
    }

    this.manager.delete(sessionId);
    return Success(undefined);
  }

  async list(): Promise<Result<string[]>> {
    return Success(this.manager.listSessions());
  }

  // Cleanup when kernel shuts down
  stop(): void {
    this.manager.stop();
  }
}

/**
 * Default progress reporter
 */
class DefaultProgressReporter implements ProgressReporter {
  constructor(private logger: Logger) {}

  start(message: string): void {
    this.logger.info(`[START] ${message}`);
  }

  update(message: string, percentage?: number): void {
    if (percentage !== undefined) {
      this.logger.info(`[${percentage}%] ${message}`);
    } else {
      this.logger.info(`[UPDATE] ${message}`);
    }
  }

  complete(message: string): void {
    this.logger.info(`[COMPLETE] ${message}`);
  }

  fail(message: string): void {
    this.logger.error(`[FAILED] ${message}`);
  }
}

/**
 * Default telemetry system
 */
class DefaultTelemetrySystem implements TelemetrySystem {
  private events: TelemetryEvent[] = [];
  private metrics = new Map<string, AggregatedMetric>();
  private maxEvents = 10000;

  track(event: TelemetryEvent): void {
    this.events.push(event);

    // Circular buffer - remove old events
    if (this.events.length > this.maxEvents) {
      this.events.shift();
    }

    // Update aggregated metrics
    this.updateMetrics(event);
  }

  private updateMetrics(event: TelemetryEvent): void {
    if (event.duration !== undefined && event.toolName) {
      const key = `${event.toolName}.duration`;
      const existing = this.metrics.get(key);

      if (existing) {
        existing.count++;
        existing.min = Math.min(existing.min, event.duration);
        existing.max = Math.max(existing.max, event.duration);
        existing.avg = (existing.avg * (existing.count - 1) + event.duration) / existing.count;
        existing.value = event.duration;
      } else {
        this.metrics.set(key, {
          name: key,
          value: event.duration,
          count: 1,
          min: event.duration,
          max: event.duration,
          avg: event.duration,
        });
      }
    }

    // Track error rates
    if (event.error && event.toolName) {
      const key = `${event.toolName}.errors`;
      const existing = this.metrics.get(key);
      if (existing) {
        existing.count++;
        existing.value++;
      } else {
        this.metrics.set(key, {
          name: key,
          value: 1,
          count: 1,
          min: 1,
          max: 1,
          avg: 1,
        });
      }
    }
  }

  getMetrics(): Map<string, AggregatedMetric> {
    return new Map(this.metrics);
  }

  getHealth(): HealthStatus {
    // Calculate error rate
    let totalErrors = 0;
    let totalCalls = 0;

    for (const [key, metric] of this.metrics) {
      if (key.endsWith('.errors')) {
        totalErrors += metric.value;
      } else if (key.endsWith('.duration')) {
        totalCalls += metric.count;
      }
    }

    const errorRate = totalCalls > 0 ? totalErrors / totalCalls : 0;

    // Calculate average latency
    let totalLatency = 0;
    let latencyCount = 0;

    for (const [key, metric] of this.metrics) {
      if (key.endsWith('.duration')) {
        totalLatency += metric.avg * metric.count;
        latencyCount += metric.count;
      }
    }

    const avgLatency = latencyCount > 0 ? totalLatency / latencyCount : 0;

    // Determine health status
    if (errorRate > 0.1 || avgLatency > 5000) {
      return {
        status: 'critical',
        issues: [
          ...(errorRate > 0.1 ? [`High error rate: ${(errorRate * 100).toFixed(1)}%`] : []),
          ...(avgLatency > 5000 ? [`High latency: ${avgLatency.toFixed(0)}ms`] : []),
        ],
        metrics: {
          errorRate,
          avgLatency,
        },
      };
    }

    if (errorRate > 0.05 || avgLatency > 2000) {
      return {
        status: 'degraded',
        issues: [
          ...(errorRate > 0.05 ? [`Elevated error rate: ${(errorRate * 100).toFixed(1)}%`] : []),
          ...(avgLatency > 2000 ? [`Elevated latency: ${avgLatency.toFixed(0)}ms`] : []),
        ],
        metrics: {
          errorRate,
          avgLatency,
        },
      };
    }

    return {
      status: 'healthy',
      metrics: {
        errorRate,
        avgLatency,
      },
    };
  }
}

// ============================================================================
// Execution Planner
// ============================================================================

/**
 * Manages execution plans and dependency resolution
 */
class ExecutionPlanner {
  constructor(private tools: Map<string, RegisteredTool>) {}

  /**
   * Build execution plan for a tool
   */
  buildPlan(toolName: string, completedSteps: Set<string>): ExecutionPlan {
    const plan: ExecutionPlan = {
      steps: [],
      dependencies: new Map(),
      completed: new Set(completedSteps),
      remaining: [],
    };

    // Build dependency graph
    const visited = new Set<string>();
    const stack: string[] = [];

    const visit = (name: string): void => {
      if (visited.has(name) || plan.completed.has(name)) {
        return;
      }

      visited.add(name);
      const tool = this.tools.get(name);

      if (!tool) {
        throw new Error(`Tool not found: ${name}`);
      }

      // Visit dependencies first
      if (tool.requires) {
        plan.dependencies.set(name, tool.requires);
        for (const dep of tool.requires) {
          if (!plan.completed.has(dep)) {
            visit(dep);
          }
        }
      }

      stack.push(name);
    };

    visit(toolName);

    // Build final plan
    plan.steps = stack;
    plan.remaining = stack.filter((step) => !plan.completed.has(step));

    return plan;
  }

  /**
   * Check if tool can be executed
   */
  canExecute(
    toolName: string,
    completedSteps: Set<string>,
  ): {
    canExecute: boolean;
    missing: string[];
  } {
    const tool = this.tools.get(toolName);
    if (!tool) {
      return { canExecute: false, missing: [toolName] };
    }

    if (!tool.requires || tool.requires.length === 0) {
      return { canExecute: true, missing: [] };
    }

    const missing = tool.requires.filter((req) => !completedSteps.has(req));
    return {
      canExecute: missing.length === 0,
      missing,
    };
  }
}

// ============================================================================
// Application Kernel Implementation
// ============================================================================

/**
 * Main kernel implementation
 */
export class ApplicationKernel implements Kernel {
  private toolRegistry: Map<string, RegisteredTool>;
  private sessionManager: SessionManager;
  private telemetry: TelemetrySystem;
  private logger: Logger;
  private planner: ExecutionPlanner;
  private policy?: Policy;
  private config: KernelConfig;
  // Simple tools are executed directly via runTool function

  constructor(options: KernelFactoryOptions) {
    this.config = options.config;
    this.toolRegistry = options.tools || new Map();
    this.sessionManager = options.sessionManager || new InMemorySessionManager();
    this.telemetry = options.telemetry || new DefaultTelemetrySystem();
    this.logger = options.logger || createLogger({ name: 'kernel' });
    this.planner = new ExecutionPlanner(this.toolRegistry);

    // Load policy if configured
    if (this.config.policyPath) {
      const policyResult = loadPolicy(this.config.policyPath, this.config.policyEnvironment);
      if (policyResult.ok) {
        this.policy = policyResult.value;
      } else {
        this.logger.warn(`Failed to load policy: ${policyResult.error}`);
      }
    }

    // No router initialization needed - using direct function execution
  }

  /**
   * Execute a tool with the unified execution path
   */
  async execute(request: ExecuteRequest): Promise<Result<unknown>> {
    const start = Date.now();
    const { toolName, params, sessionId, force } = request;

    // Track execution start
    this.telemetry.track({
      type: 'tool.execution.start',
      toolName,
      timestamp: start,
      ...(request.metadata ? { metadata: request.metadata } : {}),
    });

    try {
      // Get the tool for checking
      const tool = this.toolRegistry.get(toolName);
      if (!tool) {
        return Failure(`Tool not found: ${toolName}`);
      }

      // Check if this is a simple tool call
      if (this.isSimpleToolCall(toolName, params)) {
        this.logger.debug(`Executing simple tool directly: ${toolName}`);

        // Execute the tool directly without orchestration
        const result = await runTool(tool, params, this.logger as any);

        // If we have a session and the tool succeeded, store the result
        if (sessionId && result.ok) {
          const sessionResult = await this.sessionManager.get(sessionId);
          if (sessionResult.ok) {
            const updatedSteps = [...sessionResult.value.completed_steps];
            if (!updatedSteps.includes(toolName)) {
              updatedSteps.push(toolName);
            }

            await this.sessionManager.update(sessionId, {
              data: {
                ...sessionResult.value.data,
                [toolName]: result.value,
                [`${toolName}_result`]: result.value,
                [`${toolName}_completed`]: true,
                [`${toolName}_timestamp`]: Date.now(),
              },
              completed_steps: updatedSteps,
            });
          }
        }

        // Track completion
        this.telemetry.track({
          type: result.ok ? 'tool.execution.success' : 'tool.execution.error',
          toolName,
          timestamp: Date.now(),
          duration: Date.now() - start,
          ...(result.ok ? {} : { error: result.error }),
        });

        return result;
      }

      // Complex orchestration for tools with dependencies or policies
      // 1. Get or create session
      let session: SessionState | undefined;
      if (sessionId) {
        const sessionResult = await this.sessionManager.get(sessionId);
        if (sessionResult.ok) {
          session = sessionResult.value;
        } else if (!force) {
          return Failure(`Session not found: ${sessionId}`);
        }
      }

      // 2. Get execution plan
      const completedSteps = new Set(session?.completed_steps || []);
      const plan = this.planner.buildPlan(toolName, completedSteps);

      // 3. Execute each step in the plan
      let lastResult: Result<unknown> = Success(undefined);

      for (const stepName of plan.remaining) {
        const stepTool = this.toolRegistry.get(stepName);
        if (!stepTool) {
          return Failure(`Tool not found in registry: ${stepName}`);
        }

        // 4. Validate parameters
        const validationResult = await this.validateParams(params, stepTool.schema);
        if (!validationResult.ok) {
          return validationResult;
        }

        // 5. Apply policies
        if (this.policy) {
          const policyResults = applyPolicy(this.policy, {
            tool: stepName,
            params: validationResult.value,
          });

          const blockers = policyResults
            .filter((r) => r.matched && r.rule.actions.block)
            .map((r) => r.rule.id);

          if (blockers.length > 0) {
            return Failure(`Blocked by policies: ${blockers.join(', ')}`);
          }
        }

        // 6. Build context
        const context = await this.buildContext(stepName, session);

        // 7. Execute tool
        this.logger.debug(`Executing tool: ${stepName}`);
        const toolResult = await this.executeWithRetry(stepTool, validationResult.value, context);

        if (!toolResult.ok) {
          this.telemetry.track({
            type: 'tool.execution.error',
            toolName: stepName,
            timestamp: Date.now(),
            duration: Date.now() - start,
            error: toolResult.error,
          });
          return toolResult;
        }

        lastResult = toolResult;

        // 8. Update session with completed step
        if (session) {
          completedSteps.add(stepName);
          await this.sessionManager.update(session.sessionId, {
            completed_steps: Array.from(completedSteps),
            data: {
              ...session.data,
              [stepName]: toolResult.value,
            },
          });
        }
      }

      // Track success
      this.telemetry.track({
        type: 'tool.execution.success',
        toolName,
        timestamp: Date.now(),
        duration: Date.now() - start,
      });

      return lastResult;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);

      this.telemetry.track({
        type: 'tool.execution.error',
        toolName,
        timestamp: Date.now(),
        duration: Date.now() - start,
        error: errorMessage,
      });

      return Failure(errorMessage);
    }
  }

  /**
   * Get execution plan for a tool
   */
  async getPlan(toolName: string, sessionId?: string): Promise<string[]> {
    let completedSteps = new Set<string>();

    if (sessionId) {
      const sessionResult = await this.sessionManager.get(sessionId);
      if (sessionResult.ok) {
        completedSteps = new Set(sessionResult.value.completed_steps);
      }
    }

    const plan = this.planner.buildPlan(toolName, completedSteps);
    return plan.remaining;
  }

  /**
   * Check if a tool can be executed
   */
  async canExecute(
    toolName: string,
    sessionId?: string,
  ): Promise<{
    canExecute: boolean;
    missing: string[];
    completed: string[];
  }> {
    let completedSteps = new Set<string>();

    if (sessionId) {
      const sessionResult = await this.sessionManager.get(sessionId);
      if (sessionResult.ok) {
        completedSteps = new Set(sessionResult.value.completed_steps);
      }
    }

    const result = this.planner.canExecute(toolName, completedSteps);
    return {
      ...result,
      completed: Array.from(completedSteps),
    };
  }

  /**
   * Get all registered tools
   */
  tools(): Map<string, RegisteredTool> {
    return new Map(this.toolRegistry);
  }

  /**
   * Get a specific tool
   */
  getTool(name: string): RegisteredTool | undefined {
    return this.toolRegistry.get(name);
  }

  /**
   * Get session
   */
  async getSession(sessionId: string): Promise<Result<SessionState>> {
    return this.sessionManager.get(sessionId);
  }

  /**
   * Create new session
   */
  async createSession(): Promise<Result<SessionState>> {
    return this.sessionManager.create();
  }

  /**
   * Get health status
   */
  getHealth(): HealthStatus {
    return this.telemetry.getHealth();
  }

  /**
   * Get metrics
   */
  getMetrics(): Map<string, AggregatedMetric> {
    return this.telemetry.getMetrics();
  }

  // ============================================================================
  // Private Helper Methods
  // ============================================================================

  /**
   * Validate parameters against schema
   */
  private async validateParams(params: unknown, schema: z.ZodSchema): Promise<Result<unknown>> {
    try {
      const validated = await schema.parseAsync(params);
      return Success(validated);
    } catch (error) {
      if (error instanceof z.ZodError) {
        const issues = error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
        return Failure(`Validation failed: ${issues}`);
      }
      return Failure(`Validation error: ${String(error)}`);
    }
  }

  /**
   * Build tool context
   */
  private async buildContext(toolName: string, session?: SessionState): Promise<ToolContext> {
    const logger = createLogger({ name: toolName });
    const progress = new DefaultProgressReporter(logger);

    return {
      ...(session ? { sessionId: session.sessionId } : {}),
      session,
      logger,
      progress,
      telemetry: this.telemetry,
    } as ToolContext;
  }

  /**
   * Execute tool with retry logic
   */
  private async executeWithRetry(
    tool: RegisteredTool,
    params: unknown,
    context: ToolContext,
  ): Promise<Result<unknown>> {
    const maxRetries = this.config.maxRetries || 2;
    const retryDelay = this.config.retryDelay || 1000;

    let lastError: Error | null = null;

    for (let attempt = 0; attempt < maxRetries; attempt++) {
      try {
        const result = await tool.handler(params, context);
        return result;
      } catch (error) {
        lastError = error as Error;
        this.logger.warn(
          `Tool ${tool.name} failed (attempt ${attempt + 1}/${maxRetries}): ${lastError.message}`,
        );

        if (attempt < maxRetries - 1) {
          await new Promise((resolve) => setTimeout(resolve, retryDelay));
        }
      }
    }

    return Failure(`Tool ${tool.name} failed after ${maxRetries} attempts: ${lastError?.message}`);
  }

  /**
   * Check if this is a simple tool call that can bypass orchestration
   */
  private isSimpleToolCall(toolName: string, params: any): boolean {
    const tool = this.toolRegistry.get(toolName);
    if (!tool) return false;

    // Check if tool qualifies for simple execution
    if (!canExecuteSimply(tool, params)) {
      return false;
    }

    // Check if tool has complex policy requirements
    const hasComplexPolicy = this.policy?.rules?.some((r: any) => {
      // Check if any condition matches this tool
      const matchesTool = r.conditions?.some((c: any) => c.type === 'tool' && c.value === toolName);
      return matchesTool && r.actions && (r.actions.block || r.actions.require_approval);
    });

    // Simple tool calls have no complex policies
    return !hasComplexPolicy;
  }
}

// ============================================================================
// Factory Function
// ============================================================================

/**
 * Create a new kernel instance
 */
export async function createKernel(
  config: KernelConfig,
  tools?: Map<string, RegisteredTool>,
): Promise<Kernel> {
  const kernel = new ApplicationKernel({
    config,
    ...(tools ? { tools } : {}),
  });

  return kernel;
}

// ============================================================================
// Exports
// ============================================================================

export type {
  Kernel,
  KernelConfig,
  ExecuteRequest,
  RegisteredTool,
  ToolContext,
  SessionState,
} from './types';

export {
  InMemorySessionManager,
  DefaultProgressReporter,
  DefaultTelemetrySystem,
  ExecutionPlanner,
};
