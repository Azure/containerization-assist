/**
 * Quality Metrics Framework for LLM Integration Testing
 * Simple performance and quality measurement for LLM responses
 */

import type { LLMResponse, ConversationSession } from './llm-client-types.js';

export interface ResponseMetrics {
  responseTime: number;
  contentLength: number;
  toolCallCount: number;
  tokenUsage?: {
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
  };
  timestamp: string;
}

export interface SimpleQualityCheck {
  hasContent: boolean;
  hasToolCalls: boolean;
  responseLength: number;
  passingScore: boolean;
}

export class SimpleMetricsCollector {
  private metrics: Map<string, ResponseMetrics[]> = new Map();

  /**
   * Record response metrics from an LLM interaction
   */
  recordResponseMetrics(
    testId: string,
    response: LLMResponse,
    responseTime: number
  ): ResponseMetrics {
    const metrics: ResponseMetrics = {
      responseTime,
      contentLength: response.content.length,
      toolCallCount: response.toolCalls.length,
      tokenUsage: response.usage,
      timestamp: new Date().toISOString(),
    };

    if (!this.metrics.has(testId)) {
      this.metrics.set(testId, []);
    }
    this.metrics.get(testId)!.push(metrics);

    return metrics;
  }

  /**
   * Simple quality check for LLM response
   */
  checkQuality(response: LLMResponse, expectedToolCalls?: string[]): SimpleQualityCheck {
    const hasContent = response.content.length > 0;
    const hasToolCalls = response.toolCalls.length > 0;
    const hasExpectedTools = expectedToolCalls
      ? expectedToolCalls.some(tool =>
          response.toolCalls.some(call => call.name === tool))
      : true;

    return {
      hasContent,
      hasToolCalls,
      responseLength: response.content.length,
      passingScore: hasContent && hasExpectedTools,
    };
  }

  /**
   * Get basic summary of all metrics
   */
  getSummary(): {
    totalTests: number;
    averageResponseTime: number;
    totalTokens: number;
  } {
    const allMetrics = Array.from(this.metrics.values()).flat();

    const averageResponseTime = allMetrics.length > 0
      ? allMetrics.reduce((sum, m) => sum + m.responseTime, 0) / allMetrics.length
      : 0;

    const totalTokens = allMetrics.reduce((sum, m) =>
      sum + (m.tokenUsage?.totalTokens || 0), 0);

    return {
      totalTests: this.metrics.size,
      averageResponseTime,
      totalTokens,
    };
  }

  /**
   * Clear all collected metrics
   */
  reset(): void {
    this.metrics.clear();
  }
}

/**
 * Simple performance expectations for containerization workflows
 */
export const BASIC_PERFORMANCE_EXPECTATIONS = {
  maxResponseTime: 10000, // 10 seconds max
  minContentLength: 50,   // At least 50 characters
  dockerfileValidation: ['validate-dockerfile'],
  imageBuild: ['build-image'],
  appCreation: ['create-app'],
} as const;

/**
 * Global metrics collector instance for tests
 */
export const metricsCollector = new SimpleMetricsCollector();