/**
 * Re-export test runner types and classes from infrastructure
 * This maintains backward compatibility with existing test imports
 */

export {
  MCPTestRunner,
  TestCase,
  TestCategory,
  TestFilter,
  TestSuiteResults,
  // Re-export the extended types used internally
  type TestInfrastructureResult as TestResult,
  type TestInfrastructurePerformanceMetrics as PerformanceMetrics
} from '../infrastructure/test-runner.js';

// Also re-export the base types from result types for convenience
export type {
  TestResult as BaseTestResult,
  PerformanceMetrics as BasePerformanceMetrics
} from '../../../../src/types/result-types.js';