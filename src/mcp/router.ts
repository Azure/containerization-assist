/**
 * MCP Router - Intelligent routing for tool execution.
 *
 * Provides routing capabilities for MCP tools including:
 * - Tool registration and discovery
 * - Request routing based on tool name
 * - Middleware support for cross-cutting concerns
 * - Context propagation
 * - Error handling and recovery
 */

import type { Tool } from '@/types/index';
import type { ToolContext } from '@/mcp/context';
import type { Result } from '@/types/index';
import { Failure } from '@/types/index';
import { createLogger } from '@/lib/logger';
import type { Logger } from 'pino';

const logger = createLogger().child({ module: 'mcp-router' });

/**
 * Route handler function type.
 */
export type RouteHandler = (
  params: Record<string, unknown>,
  logger: Logger,
  context?: ToolContext,
) => Promise<Result<unknown>>;

/**
 * Middleware function type.
 */
export type Middleware = (
  params: Record<string, unknown>,
  logger: Logger,
  context: ToolContext | undefined,
  next: RouteHandler,
) => Promise<Result<unknown>>;

/**
 * Route definition.
 */
export interface Route {
  /** Tool name pattern (supports wildcards) */
  pattern: string | RegExp;
  /** Handler function */
  handler: RouteHandler;
  /** Optional description */
  description?: string;
  /** Optional schema for validation */
  schema?: Record<string, unknown>;
  /** Route-specific middleware */
  middleware?: Middleware[];
}

/**
 * Router configuration.
 */
export interface RouterConfig {
  /** Enable debug logging */
  debug?: boolean;
  /** Global middleware applied to all routes */
  globalMiddleware?: Middleware[];
  /** Error handler */
  errorHandler?: (error: Error, toolName: string, logger: Logger) => Result<unknown>;
  /** Default timeout for tool execution (ms) */
  defaultTimeout?: number;
}

/**
 * Tool metadata for registration.
 */
export interface ToolMetadata {
  name: string;
  description?: string;
  schema?: Record<string, unknown>;
  tags?: string[];
  category?: string;
}

/**
 * MCP Router implementation.
 */
export class McpRouter {
  private routes: Map<string, Route> = new Map();
  private tools: Map<string, Tool> = new Map();
  private patterns: Array<{ pattern: RegExp; route: Route }> = [];
  private config: RouterConfig;
  private globalMiddleware: Middleware[];

  constructor(config?: RouterConfig) {
    this.config = config || {};
    this.globalMiddleware = config?.globalMiddleware || [];

    if (this.config.debug) {
      logger.level = 'debug';
    }
  }

  /**
   * Register a tool with the router.
   */
  registerTool(tool: Tool): void {
    this.tools.set(tool.name, tool);

    // Create route for the tool
    const route: Route = {
      pattern: tool.name,
      handler: tool.execute,
      description: tool.description || '',
      schema: tool.schema || {},
    };

    this.routes.set(tool.name, route);

    logger.info({ tool: tool.name }, 'Registered tool with router');
  }

  /**
   * Register a route with custom handler.
   */
  registerRoute(route: Route): void {
    if (typeof route.pattern === 'string') {
      this.routes.set(route.pattern, route);
    } else {
      // RegExp pattern
      this.patterns.push({
        pattern: route.pattern,
        route,
      });
    }

    logger.info({ pattern: route.pattern.toString() }, 'Registered route');
  }

  /**
   * Register multiple tools at once.
   */
  registerTools(tools: Tool[]): void {
    tools.forEach((tool) => this.registerTool(tool));
  }

  /**
   * Find route for a tool name.
   */
  private findRoute(toolName: string): Route | undefined {
    // Check exact match first
    const exactRoute = this.routes.get(toolName);
    if (exactRoute) {
      return exactRoute;
    }

    // Check pattern matches
    for (const { pattern, route } of this.patterns) {
      if (pattern.test(toolName)) {
        return route;
      }
    }

    return undefined;
  }

  /**
   * Apply middleware chain.
   */
  private async applyMiddleware(
    params: Record<string, unknown>,
    logger: Logger,
    context: ToolContext | undefined,
    middleware: Middleware[],
    finalHandler: RouteHandler,
  ): Promise<Result<unknown>> {
    // Build middleware chain
    const chain = [...middleware].reverse().reduce(
      (next: RouteHandler, mw: Middleware): RouteHandler =>
        async (p: Record<string, unknown>, l: Logger, c?: ToolContext) =>
          mw(p, l, c || ({} as ToolContext), next),
      finalHandler,
    );

    return chain(params, logger, context);
  }

  /**
   * Execute a tool by name.
   */
  async execute(
    toolName: string,
    params: Record<string, unknown>,
    executorLogger?: Logger,
    context?: ToolContext,
  ): Promise<Result<unknown>> {
    const routeLogger = executorLogger || logger;

    try {
      routeLogger.info({ tool: toolName }, 'Routing tool execution');

      // Find route
      const route = this.findRoute(toolName);
      if (!route) {
        return Failure(`No route found for tool: ${toolName}`);
      }

      // Combine middleware
      const allMiddleware = [...this.globalMiddleware, ...(route.middleware || [])];

      // Execute with middleware chain
      const result = await this.applyMiddleware(
        params,
        routeLogger,
        context,
        allMiddleware,
        route.handler,
      );

      return result;
    } catch (error) {
      routeLogger.error({ error, tool: toolName }, 'Tool execution failed');

      // Use custom error handler if provided
      if (this.config.errorHandler) {
        return this.config.errorHandler(error as Error, toolName, routeLogger);
      }

      return Failure(`Tool execution failed: ${error}`);
    }
  }

  /**
   * Get all registered tool names.
   */
  getToolNames(): string[] {
    return Array.from(this.tools.keys());
  }

  /**
   * Get tool metadata.
   */
  getToolMetadata(toolName: string): ToolMetadata | undefined {
    const tool = this.tools.get(toolName);
    if (!tool) {
      return undefined;
    }

    return {
      name: tool.name,
      description: tool.description || '',
      schema: tool.schema || {},
    };
  }

  /**
   * List all available tools with metadata.
   */
  listTools(): ToolMetadata[] {
    return Array.from(this.tools.values()).map((tool) => ({
      name: tool.name,
      description: tool.description || '',
      schema: tool.schema || {},
    }));
  }

  /**
   * Check if a tool exists.
   */
  hasTool(toolName: string): boolean {
    return this.tools.has(toolName) || this.findRoute(toolName) !== undefined;
  }

  /**
   * Create a sub-router with isolated routes.
   */
  createSubRouter(prefix: string): SubRouter {
    return new SubRouter(this, prefix);
  }

  /**
   * Mount a sub-router.
   */
  mount(prefix: string, subRouter: SubRouter): void {
    subRouter.getRoutes().forEach((route) => {
      const prefixedRoute = {
        ...route,
        pattern: `${prefix}:${route.pattern}`,
      };
      this.registerRoute(prefixedRoute);
    });
  }
}

/**
 * Sub-router for organizing related routes.
 */
export class SubRouter {
  private parent: McpRouter;
  private prefix: string;
  private routes: Route[] = [];

  constructor(parent: McpRouter, prefix: string) {
    this.parent = parent;
    this.prefix = prefix;
  }

  /**
   * Register a route in the sub-router.
   */
  registerRoute(route: Route): void {
    this.routes.push(route);
  }

  /**
   * Get all routes in the sub-router.
   */
  getRoutes(): Route[] {
    return this.routes;
  }

  /**
   * Get the parent router.
   */
  getParent(): McpRouter {
    return this.parent;
  }

  /**
   * Get the prefix.
   */
  getPrefix(): string {
    return this.prefix;
  }
}

/**
 * Common middleware implementations.
 */
export const Middleware = {
  /**
   * Logging middleware.
   */
  logging(): Middleware {
    return async (params, logger, context, next) => {
      const startTime = Date.now();
      logger.info({ params }, 'Tool execution started');

      const result = await next(params, logger, context);

      const duration = Date.now() - startTime;
      logger.info(
        {
          success: result.ok,
          duration,
        },
        'Tool execution completed',
      );

      return result;
    };
  },

  /**
   * Validation middleware.
   */
  validation(_schema: any): Middleware {
    return async (params, logger, context, next) => {
      try {
        // Basic validation (can be enhanced with Zod)
        // TODO: Implement schema validation when needed
        if (!params || typeof params !== 'object') {
          return Failure('Invalid parameters: expected object');
        }

        return next(params, logger, context);
      } catch (error) {
        return Failure(`Validation failed: ${error}`);
      }
    };
  },

  /**
   * Timeout middleware.
   */
  timeout(ms: number): Middleware {
    return async (params, logger, context, next) => {
      const timeoutPromise = new Promise<Result<unknown>>((_, reject) =>
        setTimeout(() => reject(new Error('Execution timeout')), ms),
      );

      try {
        const result = await Promise.race([next(params, logger, context), timeoutPromise]);
        return result;
      } catch (error) {
        return Failure(`Execution timed out after ${ms}ms`);
      }
    };
  },

  /**
   * Retry middleware.
   */
  retry(attempts: number = 3, delay: number = 1000): Middleware {
    return async (params, logger, context, next) => {
      let lastError: string = '';

      for (let i = 0; i < attempts; i++) {
        if (i > 0) {
          logger.info({ attempt: i + 1, maxAttempts: attempts }, 'Retrying tool execution');
          await new Promise((resolve) => setTimeout(resolve, delay * i));
        }

        const result = await next(params, logger, context);
        if (result.ok) {
          return result;
        }

        lastError = result.error;
      }

      return Failure(`Failed after ${attempts} attempts: ${lastError}`);
    };
  },

  /**
   * Cache middleware.
   */
  cache(ttl: number = 60000): Middleware {
    const cache = new Map<string, { result: Result<unknown>; expires: number }>();

    return async (params, logger, context, next) => {
      const key = JSON.stringify(params);
      const cached = cache.get(key);

      if (cached && cached.expires > Date.now()) {
        logger.debug('Returning cached result');
        return cached.result;
      }

      const result = await next(params, logger, context);

      if (result.ok) {
        cache.set(key, {
          result,
          expires: Date.now() + ttl,
        });
      }

      return result;
    };
  },
};

/**
 * Create a default router with common configuration.
 */
export function createDefaultRouter(config?: RouterConfig): McpRouter {
  return new McpRouter({
    ...config,
    globalMiddleware: [Middleware.logging(), ...(config?.globalMiddleware || [])],
  });
}
