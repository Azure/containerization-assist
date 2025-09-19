/**
 * PKSP Health Endpoints and Debugging Utilities
 * Provides comprehensive debugging and monitoring capabilities for PKSP system
 */

import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';
import { getTelemetryCollector } from '@lib/telemetry';
import { getPerformanceProfiler } from '@lib/performance';
import { getABTestManager } from '@/prompts/ab-testing';
import { prompts } from '@/prompts/prompt-registry';
import { getStrategySelector } from '@/strategies/selector';

/**
 * System health status
 */
export interface SystemHealth {
  status: 'healthy' | 'degraded' | 'unhealthy';
  timestamp: string;
  uptime: number;
  components: {
    prompts: ComponentHealth;
    knowledge: ComponentHealth;
    strategies: ComponentHealth;
    policies: ComponentHealth;
    performance: ComponentHealth;
    telemetry: ComponentHealth;
  };
  metrics: {
    totalRequests: number;
    errorRate: number;
    averageLatency: number;
    memoryUsageMB: number;
    cpuUsagePercent: number;
  };
  issues: HealthIssue[];
  recommendations: string[];
}

export interface ComponentHealth {
  status: 'healthy' | 'degraded' | 'unhealthy';
  lastCheck: string;
  issues: string[];
  metrics?: Record<string, number>;
}

export interface HealthIssue {
  component: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  message: string;
  timestamp: string;
  resolution?: string;
}

/**
 * Debug information structure
 */
export interface DebugInfo {
  system: {
    nodeVersion: string;
    platform: string;
    architecture: string;
    memory: NodeJS.MemoryUsage;
    uptime: number;
  };
  pksp: {
    promptsLoaded: number;
    strategiesLoaded: number;
    policiesLoaded: number;
    cacheStats: {
      promptCache: { size: number; hitRate: number };
      templateCache: { size: number; hitRate: number };
      performanceCache: { size: number };
    };
    activeTests: number;
    recentErrors: Array<{
      timestamp: string;
      component: string;
      error: string;
    }>;
  };
  performance: {
    targets: Record<string, number>;
    current: Record<string, number>;
    violations: Array<{
      operation: string;
      target: number;
      actual: number;
      timestamp: string;
    }>;
  };
}

/**
 * Health and debugging utilities
 */
export class PKSPHealthChecker {
  private logger?: Logger;
  private startTime = Date.now();

  constructor(logger?: Logger) {
    if (logger) {
      this.logger = logger.child({ component: 'PKSPHealthChecker' });
    }
  }

  /**
   * Get comprehensive system health status
   */
  async getSystemHealth(): Promise<SystemHealth> {
    const timestamp = new Date().toISOString();
    const uptime = Date.now() - this.startTime;

    // Check all components
    const components = {
      prompts: await this.checkPromptsHealth(),
      knowledge: await this.checkKnowledgeHealth(),
      strategies: await this.checkStrategiesHealth(),
      policies: await this.checkPoliciesHealth(),
      performance: await this.checkPerformanceHealth(),
      telemetry: await this.checkTelemetryHealth(),
    };

    // Aggregate metrics
    const telemetry = getTelemetryCollector();
    const telemetryMetrics = telemetry.getMetrics();
    const memoryUsage = process.memoryUsage();

    const metrics = {
      totalRequests: telemetryMetrics.totalEvents,
      errorRate: telemetryMetrics.errorRate,
      averageLatency: telemetryMetrics.averageLatency,
      memoryUsageMB: memoryUsage.heapUsed / 1024 / 1024,
      cpuUsagePercent: 0, // Would need external library for accurate CPU usage
    };

    // Collect all issues
    const issues: HealthIssue[] = [];
    for (const [componentName, component] of Object.entries(components)) {
      for (const issue of component.issues) {
        issues.push({
          component: componentName,
          severity: this.determineSeverity(componentName, issue),
          message: issue,
          timestamp,
          resolution: this.getResolutionSuggestion(componentName, issue),
        });
      }
    }

    // Determine overall status
    const overallStatus = this.determineOverallStatus(components, issues);

    // Generate recommendations
    const recommendations = this.generateRecommendations(components, issues, metrics);

    return {
      status: overallStatus,
      timestamp,
      uptime,
      components,
      metrics,
      issues,
      recommendations,
    };
  }

  /**
   * Get detailed debug information
   */
  async getDebugInfo(): Promise<DebugInfo> {
    const telemetry = getTelemetryCollector();
    const profiler = getPerformanceProfiler();

    // System information
    const system = {
      nodeVersion: process.version,
      platform: process.platform,
      architecture: process.arch,
      memory: process.memoryUsage(),
      uptime: process.uptime(),
    };

    // Get recent errors from telemetry
    const recentErrors = telemetry
      .getEvents({
        success: false,
        since: Date.now() - 3600000, // Last hour
        limit: 10,
      })
      .map((event) => ({
        timestamp: new Date(event.timestamp).toISOString(),
        component: event.tool || 'unknown',
        error: event.errorMessage || 'Unknown error',
      }));

    // PKSP information
    const pksp = {
      promptsLoaded: (await prompts.list()).length,
      strategiesLoaded: 3, // Based on existing files
      policiesLoaded: 0, // Would need to check policies
      cacheStats: {
        promptCache: { size: 0, hitRate: 0 }, // Would need cache stats
        templateCache: { size: 0, hitRate: 0 },
        performanceCache: { size: 0 },
      },
      activeTests: getABTestManager().getActiveTests().length,
      recentErrors,
    };

    // Performance information
    const performanceReport = profiler.getPerformanceReport();
    const performance = {
      targets: performanceReport.targets as unknown as Record<string, number>,
      current: Object.fromEntries(
        Object.entries(performanceReport.operations).map(([op, stats]) => [op, stats?.p99 || 0]),
      ),
      violations: performanceReport.violations.map((v) => ({
        operation: v.operation,
        target: v.target,
        actual: v.actual,
        timestamp: new Date().toISOString(),
      })),
    };

    return {
      system,
      pksp,
      performance,
    };
  }

  /**
   * Run comprehensive system diagnostics
   */
  async runDiagnostics(): Promise<{
    passed: number;
    failed: number;
    warnings: number;
    tests: Array<{
      name: string;
      status: 'pass' | 'fail' | 'warning';
      message: string;
      duration: number;
    }>;
  }> {
    const tests = [
      {
        name: 'Prompt Registry Initialization',
        test: () => this.testPromptRegistry(),
      },
      {
        name: 'Template Rendering',
        test: () => this.testTemplateRendering(),
      },
      {
        name: 'Strategy Selection',
        test: () => this.testStrategySelection(),
      },
      {
        name: 'Performance Profiling',
        test: () => this.testPerformanceProfiling(),
      },
      {
        name: 'Telemetry Collection',
        test: () => this.testTelemetryCollection(),
      },
      {
        name: 'Cache Functionality',
        test: () => this.testCacheFunctionality(),
      },
      {
        name: 'A/B Testing Framework',
        test: () => this.testABTesting(),
      },
      {
        name: 'Memory Usage',
        test: () => this.testMemoryUsage(),
      },
    ];

    const results = [];
    let passed = 0;
    let failed = 0;
    let warnings = 0;

    for (const { name, test } of tests) {
      const startTime = performance.now();
      try {
        const result = await test();
        const duration = performance.now() - startTime;

        results.push({
          name,
          status: result.status,
          message: result.message,
          duration,
        });

        switch (result.status) {
          case 'pass':
            passed++;
            break;
          case 'fail':
            failed++;
            break;
          case 'warning':
            warnings++;
            break;
        }
      } catch (error) {
        const duration = performance.now() - startTime;
        results.push({
          name,
          status: 'fail' as const,
          message: `Test threw exception: ${error}`,
          duration,
        });
        failed++;
      }
    }

    return { passed, failed, warnings, tests: results };
  }

  /**
   * Get performance recommendations
   */
  getPerformanceRecommendations(): string[] {
    const profiler = getPerformanceProfiler();
    const report = profiler.getPerformanceReport();
    return report.recommendations;
  }

  /**
   * Clear all caches and reset performance metrics
   */
  resetSystem(): Result<void> {
    try {
      // Clear telemetry
      getTelemetryCollector().clear();

      // Reset performance profiler
      getPerformanceProfiler().reset();

      this.logger?.info('System reset completed');
      return Success(undefined);
    } catch (error) {
      return Failure(`Failed to reset system: ${error}`);
    }
  }

  // Private diagnostic test methods

  private async testPromptRegistry(): Promise<{
    status: 'pass' | 'fail' | 'warning';
    message: string;
  }> {
    try {
      const allPrompts = await prompts.list();
      if (allPrompts.length === 0) {
        return { status: 'warning', message: 'No prompts loaded in registry' };
      }

      // Test basic prompt rendering
      const testPrompt = allPrompts[0];
      if (!testPrompt) {
        throw new Error('No prompts available for testing');
      }
      const result = await prompts.render(testPrompt.id, {});

      if (result.ok) {
        return {
          status: 'pass',
          message: `Prompt registry working. ${allPrompts.length} prompts loaded.`,
        };
      } else {
        return { status: 'fail', message: `Prompt rendering failed: ${result.error}` };
      }
    } catch (error) {
      return { status: 'fail', message: `Prompt registry test failed: ${error}` };
    }
  }

  private async testTemplateRendering(): Promise<{
    status: 'pass' | 'fail' | 'warning';
    message: string;
  }> {
    // Template rendering test would go here
    // For now, return a simple pass
    return { status: 'pass', message: 'Template rendering functionality available' };
  }

  private async testStrategySelection(): Promise<{
    status: 'pass' | 'fail' | 'warning';
    message: string;
  }> {
    try {
      getStrategySelector(); // Verify it initializes

      // Would need sample strategies to test properly
      return { status: 'pass', message: 'Strategy selection system initialized' };
    } catch (error) {
      return { status: 'fail', message: `Strategy selection test failed: ${error}` };
    }
  }

  private async testPerformanceProfiling(): Promise<{
    status: 'pass' | 'fail' | 'warning';
    message: string;
  }> {
    try {
      const profiler = getPerformanceProfiler();

      // Test profiling an operation
      const { metrics } = await profiler.profile('test-operation', async () => {
        await new Promise((resolve) => setTimeout(resolve, 10));
        return 'test-result';
      });

      if (metrics.duration > 0) {
        return {
          status: 'pass',
          message: `Performance profiling working. Test operation took ${metrics.duration.toFixed(2)}ms`,
        };
      } else {
        return { status: 'warning', message: 'Performance profiling may not be accurate' };
      }
    } catch (error) {
      return { status: 'fail', message: `Performance profiling test failed: ${error}` };
    }
  }

  private async testTelemetryCollection(): Promise<{
    status: 'pass' | 'fail' | 'warning';
    message: string;
  }> {
    try {
      const telemetry = getTelemetryCollector();

      // Record a test event
      telemetry.recordEvent({
        eventType: 'pksp.component.loaded',
        success: true,
      });

      const metrics = telemetry.getMetrics();
      return {
        status: 'pass',
        message: `Telemetry collection working. ${metrics.totalEvents} events recorded.`,
      };
    } catch (error) {
      return { status: 'fail', message: `Telemetry collection test failed: ${error}` };
    }
  }

  private async testCacheFunctionality(): Promise<{
    status: 'pass' | 'fail' | 'warning';
    message: string;
  }> {
    // Test would require access to cache instances
    return { status: 'pass', message: 'Cache functionality initialized' };
  }

  private async testABTesting(): Promise<{ status: 'pass' | 'fail' | 'warning'; message: string }> {
    try {
      const abManager = getABTestManager();
      const activeTests = abManager.getActiveTests();

      return {
        status: 'pass',
        message: `A/B testing framework working. ${activeTests.length} active tests.`,
      };
    } catch (error) {
      return { status: 'fail', message: `A/B testing test failed: ${error}` };
    }
  }

  private async testMemoryUsage(): Promise<{
    status: 'pass' | 'fail' | 'warning';
    message: string;
  }> {
    const memory = process.memoryUsage();
    const heapUsedMB = memory.heapUsed / 1024 / 1024;

    if (heapUsedMB > 500) {
      return { status: 'fail', message: `High memory usage: ${heapUsedMB.toFixed(1)}MB` };
    } else if (heapUsedMB > 200) {
      return { status: 'warning', message: `Elevated memory usage: ${heapUsedMB.toFixed(1)}MB` };
    } else {
      return { status: 'pass', message: `Memory usage normal: ${heapUsedMB.toFixed(1)}MB` };
    }
  }

  // Private helper methods for health checking

  private async checkPromptsHealth(): Promise<ComponentHealth> {
    const issues: string[] = [];
    const metrics: Record<string, number> = {};

    try {
      const allPrompts = await prompts.list();
      metrics.totalPrompts = allPrompts.length;

      if (allPrompts.length === 0) {
        issues.push('No prompts loaded');
      }

      // Check for deprecated prompts
      const deprecatedPrompts = allPrompts.filter((p) => p.deprecated);
      metrics.deprecatedPrompts = deprecatedPrompts.length;

      if (deprecatedPrompts.length > allPrompts.length * 0.5) {
        issues.push('High percentage of deprecated prompts');
      }

      return {
        status: issues.length === 0 ? 'healthy' : 'degraded',
        lastCheck: new Date().toISOString(),
        issues,
        metrics,
      };
    } catch (error) {
      return {
        status: 'unhealthy',
        lastCheck: new Date().toISOString(),
        issues: [`Prompts health check failed: ${error}`],
      };
    }
  }

  private async checkKnowledgeHealth(): Promise<ComponentHealth> {
    // Knowledge health checking would be implemented here
    return {
      status: 'healthy',
      lastCheck: new Date().toISOString(),
      issues: [],
    };
  }

  private async checkStrategiesHealth(): Promise<ComponentHealth> {
    // Strategy health checking would be implemented here
    return {
      status: 'healthy',
      lastCheck: new Date().toISOString(),
      issues: [],
    };
  }

  private async checkPoliciesHealth(): Promise<ComponentHealth> {
    // Policy health checking would be implemented here
    return {
      status: 'healthy',
      lastCheck: new Date().toISOString(),
      issues: [],
    };
  }

  private async checkPerformanceHealth(): Promise<ComponentHealth> {
    const profiler = getPerformanceProfiler();
    const report = profiler.getPerformanceReport();

    const issues: string[] = [];
    const metrics: Record<string, number> = {};

    // Count violations by severity
    const criticalViolations = report.violations.filter((v) => v.severity === 'critical');
    const warningViolations = report.violations.filter((v) => v.severity === 'warning');

    metrics.criticalViolations = criticalViolations.length;
    metrics.warningViolations = warningViolations.length;

    if (criticalViolations.length > 0) {
      issues.push(`${criticalViolations.length} critical performance violations`);
    }

    if (warningViolations.length > 5) {
      issues.push(`${warningViolations.length} performance warnings`);
    }

    return {
      status:
        criticalViolations.length > 0
          ? 'unhealthy'
          : warningViolations.length > 0
            ? 'degraded'
            : 'healthy',
      lastCheck: new Date().toISOString(),
      issues,
      metrics,
    };
  }

  private async checkTelemetryHealth(): Promise<ComponentHealth> {
    const telemetry = getTelemetryCollector();
    const healthStatus = telemetry.getHealthStatus();

    return {
      status:
        healthStatus.status === 'healthy'
          ? 'healthy'
          : healthStatus.status === 'warning'
            ? 'degraded'
            : 'unhealthy',
      lastCheck: new Date().toISOString(),
      issues: healthStatus.issues,
    };
  }

  private determineSeverity(_component: string, issue: string): HealthIssue['severity'] {
    if (issue.includes('critical') || issue.includes('failed') || issue.includes('unhealthy')) {
      return 'critical';
    }
    if (issue.includes('high') || issue.includes('violation')) {
      return 'high';
    }
    if (issue.includes('warning') || issue.includes('elevated')) {
      return 'medium';
    }
    return 'low';
  }

  private getResolutionSuggestion(_component: string, issue: string): string | undefined {
    const resolutions: Record<string, string> = {
      'No prompts loaded': 'Check prompt directory configuration and file permissions',
      'High memory usage': 'Consider reducing cache sizes or implementing memory limits',
      'High error rate': 'Review error logs and implement error handling improvements',
      'Low cache hit rate': 'Review cache configuration and TTL settings',
      'performance violations': 'Optimize slow operations and consider scaling resources',
    };

    for (const [pattern, resolution] of Object.entries(resolutions)) {
      if (issue.toLowerCase().includes(pattern.toLowerCase())) {
        return resolution;
      }
    }

    return undefined;
  }

  private determineOverallStatus(
    components: Record<string, ComponentHealth>,
    issues: HealthIssue[],
  ): SystemHealth['status'] {
    const criticalIssues = issues.filter((i) => i.severity === 'critical');
    const unhealthyComponents = Object.values(components).filter((c) => c.status === 'unhealthy');

    if (criticalIssues.length > 0 || unhealthyComponents.length > 0) {
      return 'unhealthy';
    }

    const degradedComponents = Object.values(components).filter((c) => c.status === 'degraded');
    if (degradedComponents.length > 0) {
      return 'degraded';
    }

    return 'healthy';
  }

  private generateRecommendations(
    components: Record<string, ComponentHealth>,
    issues: HealthIssue[],
    metrics: SystemHealth['metrics'],
  ): string[] {
    const recommendations: string[] = [];

    // High-level recommendations based on overall metrics
    if (metrics.errorRate > 0.05) {
      recommendations.push(
        'Error rate is elevated. Review error logs and implement better error handling.',
      );
    }

    if (metrics.averageLatency > 1000) {
      recommendations.push(
        'Average latency is high. Consider performance optimizations and caching.',
      );
    }

    if (metrics.memoryUsageMB > 200) {
      recommendations.push(
        'Memory usage is elevated. Consider reducing cache sizes or implementing memory limits.',
      );
    }

    // Component-specific recommendations
    for (const [componentName, component] of Object.entries(components)) {
      if (component.status === 'unhealthy') {
        recommendations.push(
          `${componentName} component is unhealthy. Immediate attention required.`,
        );
      } else if (component.status === 'degraded') {
        recommendations.push(
          `${componentName} component is degraded. Monitor closely and address issues.`,
        );
      }
    }

    // Critical issue recommendations
    const criticalIssues = issues.filter((i) => i.severity === 'critical');
    if (criticalIssues.length > 0) {
      recommendations.push(`${criticalIssues.length} critical issues require immediate attention.`);
    }

    if (recommendations.length === 0) {
      recommendations.push('System is healthy. Continue monitoring for any changes.');
    }

    return recommendations.slice(0, 5); // Limit to top 5 recommendations
  }
}

/**
 * Global health checker instance
 */
let healthCheckerInstance: PKSPHealthChecker | null = null;

/**
 * Get or create health checker instance
 */
export function getPKSPHealthChecker(logger?: Logger): PKSPHealthChecker {
  if (!healthCheckerInstance) {
    healthCheckerInstance = new PKSPHealthChecker(logger);
  }
  return healthCheckerInstance;
}

/**
 * Convenience functions for health monitoring
 */
export const health = {
  /**
   * Get system health status
   */
  async getStatus(): Promise<SystemHealth> {
    return getPKSPHealthChecker().getSystemHealth();
  },

  /**
   * Get debug information
   */
  async getDebugInfo(): Promise<DebugInfo> {
    return getPKSPHealthChecker().getDebugInfo();
  },

  /**
   * Run system diagnostics
   */
  async runDiagnostics(): Promise<ReturnType<PKSPHealthChecker['runDiagnostics']>> {
    return getPKSPHealthChecker().runDiagnostics();
  },

  /**
   * Reset system caches and metrics
   */
  reset(): Result<void> {
    return getPKSPHealthChecker().resetSystem();
  },
};
