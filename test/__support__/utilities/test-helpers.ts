import { jest } from '@jest/globals';
import type { Logger } from 'pino';

export interface TestContext {
  logger: Logger;
}

export function createTestContext(overrides?: Partial<any>): TestContext {
  const logger = createMockLogger();
  return { logger };
}

export async function cleanupTestContext(context: TestContext): Promise<void> {
  // Cleanup not needed for mock logger
}

export function createMockLogger(): Logger {
  // Create a proper recursive mock logger matching pino's interface
  const mockLogger: any = {
    debug: jest.fn(),
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    fatal: jest.fn(),
    trace: jest.fn(),
    silent: jest.fn(),
    child: jest.fn(() => createMockLogger()),
    level: 'info'
  };
  
  return mockLogger as Logger;
}

/**
 * Helper to wait for async operations to complete
 */
export async function waitFor(
  condition: () => boolean | Promise<boolean>,
  timeout = 5000,
  interval = 100
): Promise<void> {
  const start = Date.now();
  
  while (Date.now() - start < timeout) {
    const result = await condition();
    if (result) {
      return;
    }
    await new Promise(resolve => setTimeout(resolve, interval));
  }
  
  throw new Error(`Condition not met within ${timeout}ms`);
}

/**
 * Helper to create mock performance metrics
 */
export function createMockPerformanceMetrics() {
  return {
    duration: Math.random() * 100,
    memory: Math.random() * 50,
    cpu: Math.random() * 10,
    timestamp: new Date().toISOString()
  };
}

/**
 * Helper to create mock session data
 */
export function createMockSessionData(overrides?: Record<string, any>) {
  return {
    id: `test-session-${Date.now()}`,
    status: 'active',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    metadata: {},
    ...overrides
  };
}

/**
 * Helper to measure execution time
 */
export async function measureTime<T>(fn: () => Promise<T>): Promise<{ result: T; duration: number }> {
  const start = Date.now();
  const result = await fn();
  const duration = Date.now() - start;
  return { result, duration };
}

/**
 * Performance testing utilities
 */
export interface PerformanceStatistics {
  min: number;
  max: number;
  median: number;
  mean: number;
  p95: number;
  p99: number;
}

export function calculateStatistics(measurements: number[]): PerformanceStatistics {
  const sorted = [...measurements].sort((a, b) => a - b);
  const len = sorted.length;

  return {
    min: sorted[0],
    max: sorted[len - 1],
    median: len % 2 === 0 ? (sorted[len / 2 - 1] + sorted[len / 2]) / 2 : sorted[Math.floor(len / 2)],
    mean: measurements.reduce((sum, val) => sum + val, 0) / len,
    p95: sorted[Math.floor(len * 0.95)],
    p99: sorted[Math.floor(len * 0.99)]
  };
}

export function determinePerformanceStatus(
  current: number, 
  excellent: number, 
  good: number, 
  warning: number,
  lowerIsBetter = false
): 'excellent' | 'good' | 'warning' | 'critical' {
  if (lowerIsBetter) {
    if (current <= excellent) return 'excellent';
    if (current <= good) return 'good';
    if (current <= warning) return 'warning';
    return 'critical';
  } else {
    if (current >= excellent) return 'excellent';
    if (current >= good) return 'good';
    if (current >= warning) return 'warning';
    return 'critical';
  }
}

/**
 * Create a mock performance benchmark result
 */
export interface BenchmarkResult {
  name: string;
  category: string;
  duration: number;
  status: 'excellent' | 'good' | 'warning' | 'critical';
  baseline: number;
  target: number;
}

export function createMockBenchmark(overrides?: Partial<BenchmarkResult>): BenchmarkResult {
  const duration = Math.random() * 50 + 10;
  const baseline = 100;
  const target = 50;
  
  return {
    name: 'test-benchmark',
    category: 'performance',
    duration,
    status: determinePerformanceStatus(duration, target, baseline, baseline * 2, true),
    baseline,
    target,
    ...overrides
  };
}

/**
 * ESM Mock Creation and Result Testing Utilities
 */
import type { Result } from "../../src/types"

export function expectSuccess<T>(result: Result<T>): T {
  if (result.kind !== 'ok') {
    throw new Error(`Expected success but got failure: ${result.error}`);
  }
  return result.value;
}

export function expectFailure<T>(result: Result<T>): string {
  if (result.ok) {
    throw new Error(`Expected failure but got success: ${JSON.stringify(result.value)}`);
  }
  return result.error;
}

export async function waitForMockCall(mockFn: jest.MockedFunction<any>, timeout = 5000): Promise<void> {
  const start = Date.now();
  while (mockFn.mock.calls.length === 0) {
    if (Date.now() - start > timeout) {
      throw new Error('Mock function was not called within timeout');
    }
    await new Promise(resolve => setTimeout(resolve, 10));
  }
}

export function resetAllMocks(...mocks: jest.MockedFunction<any>[]) {
  mocks.forEach(mock => mock.mockClear());
}

export function createESMMock<T extends Record<string, any>>(
  mockImplementation: Partial<T>
): T {
  const mock = {} as T;
  Object.keys(mockImplementation).forEach(key => {
    const value = mockImplementation[key as keyof T];
    if (typeof value === 'function') {
      mock[key as keyof T] = jest.fn().mockImplementation(value) as any;
    } else {
      mock[key as keyof T] = value;
    }
  });
  return mock;
}