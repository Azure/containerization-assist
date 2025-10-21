/**
 * Simple test to verify our metrics simplification works
 */

import { describe, it, expect } from '@jest/globals';
import { SimpleMetricsCollector, BASIC_PERFORMANCE_EXPECTATIONS, metricsCollector } from './quality-metrics.js';
import type { LLMResponse } from './llm-client-types.js';

describe('Simplified Quality Metrics', () => {
  it('should record basic response metrics', () => {
    const collector = new SimpleMetricsCollector();

    const mockResponse: LLMResponse = {
      content: 'This is a test response from the LLM',
      toolCalls: [{ id: '1', name: 'validate-dockerfile', arguments: {} }],
      finishReason: 'stop',
      usage: {
        promptTokens: 10,
        completionTokens: 20,
        totalTokens: 30,
      },
    };

    const metrics = collector.recordResponseMetrics('test-1', mockResponse, 500);

    expect(metrics.responseTime).toBe(500);
    expect(metrics.contentLength).toBe(mockResponse.content.length);
    expect(metrics.toolCallCount).toBe(1);
    expect(metrics.tokenUsage).toEqual(mockResponse.usage);
  });

  it('should perform basic quality checks', () => {
    const collector = new SimpleMetricsCollector();

    const mockResponse: LLMResponse = {
      content: 'This response includes tool usage',
      toolCalls: [{ id: '1', name: 'validate-dockerfile', arguments: {} }],
      finishReason: 'tool_calls',
    };

    const quality = collector.checkQuality(mockResponse, ['validate-dockerfile']);

    expect(quality.hasContent).toBe(true);
    expect(quality.hasToolCalls).toBe(true);
    expect(quality.passingScore).toBe(true);
    expect(quality.responseLength).toBeGreaterThan(0);
  });

  it('should provide simple performance expectations', () => {
    expect(BASIC_PERFORMANCE_EXPECTATIONS.maxResponseTime).toBe(10000);
    expect(BASIC_PERFORMANCE_EXPECTATIONS.minContentLength).toBe(50);
    expect(BASIC_PERFORMANCE_EXPECTATIONS.dockerfileValidation).toEqual(['validate-dockerfile']);
  });

  it('should have global metrics collector instance', () => {
    expect(metricsCollector).toBeInstanceOf(SimpleMetricsCollector);

    // Should be able to use global instance
    metricsCollector.reset(); // Should not throw
    const summary = metricsCollector.getSummary();
    expect(summary.totalTests).toBe(0);
  });
});