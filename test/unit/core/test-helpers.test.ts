/**
 * Tests for test utilities
 */

import {
  createMockLogger,
  measureTime,
  calculateStatistics,
  determinePerformanceStatus,
  createMockBenchmark,
  waitFor
} from '../../__support__/utilities/test-helpers';

describe('Test Utilities', () => {
  describe('createMockLogger', () => {
    it('should create a mock logger with all required methods', () => {
      const logger = createMockLogger();
      
      expect(logger.info).toBeDefined();
      expect(logger.warn).toBeDefined();
      expect(logger.error).toBeDefined();
      expect(logger.debug).toBeDefined();
      expect(logger.child).toBeDefined();
    });

    it('should create child loggers that are also mocks', () => {
      const logger = createMockLogger();
      const child = logger.child({ test: true });
      
      expect(child.info).toBeDefined();
      expect(child.child).toBeDefined();
    });
  });

  describe('measureTime', () => {
    it('should measure execution time of async functions', async () => {
      const { result, duration } = await measureTime(async () => {
        await new Promise(resolve => setTimeout(resolve, 10));
        return 'test result';
      });
      
      expect(result).toBe('test result');
      expect(duration).toBeGreaterThanOrEqual(8); // Allow for timing variations
      expect(duration).toBeLessThan(100); // Should be reasonable
    });
  });

  describe('calculateStatistics', () => {
    it('should calculate correct statistics', () => {
      const measurements = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
      const stats = calculateStatistics(measurements);
      
      expect(stats.min).toBe(1);
      expect(stats.max).toBe(10);
      expect(stats.mean).toBe(5.5);
      expect(stats.median).toBe(5.5);
      expect(stats.p95).toBe(10);
    });

    it('should handle odd number of measurements', () => {
      const measurements = [1, 3, 5];
      const stats = calculateStatistics(measurements);
      
      expect(stats.median).toBe(3);
    });
  });

  describe('determinePerformanceStatus', () => {
    it('should classify performance correctly when higher is better', () => {
      expect(determinePerformanceStatus(100, 90, 70, 50, false)).toBe('excellent');
      expect(determinePerformanceStatus(80, 90, 70, 50, false)).toBe('good');
      expect(determinePerformanceStatus(60, 90, 70, 50, false)).toBe('warning');
      expect(determinePerformanceStatus(40, 90, 70, 50, false)).toBe('critical');
    });

    it('should classify performance correctly when lower is better', () => {
      expect(determinePerformanceStatus(40, 50, 70, 90, true)).toBe('excellent');
      expect(determinePerformanceStatus(60, 50, 70, 90, true)).toBe('good');
      expect(determinePerformanceStatus(80, 50, 70, 90, true)).toBe('warning');
      expect(determinePerformanceStatus(100, 50, 70, 90, true)).toBe('critical');
    });
  });

  describe('createMockBenchmark', () => {
    it('should create a valid benchmark result', () => {
      const benchmark = createMockBenchmark();
      
      expect(benchmark.name).toBe('test-benchmark');
      expect(benchmark.category).toBe('performance');
      expect(benchmark.duration).toBeGreaterThan(0);
      expect(benchmark.baseline).toBeDefined();
      expect(benchmark.target).toBeDefined();
      expect(['excellent', 'good', 'warning', 'critical']).toContain(benchmark.status);
    });

    it('should accept overrides', () => {
      const benchmark = createMockBenchmark({
        name: 'custom-benchmark',
        duration: 25
      });
      
      expect(benchmark.name).toBe('custom-benchmark');
      expect(benchmark.duration).toBe(25);
    });
  });

  describe('waitFor', () => {
    it('should resolve when condition becomes true', async () => {
      let condition = false;
      setTimeout(() => { condition = true; }, 50);
      
      await expect(waitFor(() => condition, 200, 10)).resolves.toBeUndefined();
    });

    it('should timeout when condition never becomes true', async () => {
      await expect(waitFor(() => false, 100, 10)).rejects.toThrow('Condition not met within 100ms');
    });
  });
});