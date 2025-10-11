import { createRequire } from 'module';
const require = createRequire(import.meta.url);

const commonModuleNameMapper = {
  // Path aliases from tsconfig
  '^@/(.*)$': '<rootDir>/src/$1',

  // Handle .js imports and map them to .ts
  '^(\\.{1,2}/.*)\\.js$': '$1',

  // Test support mappings
  '^@test/fixtures/(.*)$': '<rootDir>/test/__support__/fixtures/$1',
  '^@test/utilities/(.*)$': '<rootDir>/test/__support__/utilities/$1',
  '^@test/mocks/(.*)$': '<rootDir>/test/__support__/mocks/$1',
};

const commonTsConfig = {
  module: 'ES2022',
  moduleResolution: 'bundler',
  target: 'ES2022',
  allowSyntheticDefaultImports: true,
  esModuleInterop: true,
  isolatedModules: true,
};

const commonTransform = {
  '^.+\\.tsx?$': [
    'ts-jest',
    {
      useESM: true,
      tsconfig: commonTsConfig,
    },
  ],
};

/** @type {import('jest').Config} */
export default {
  preset: 'ts-jest/presets/default-esm',
  testEnvironment: 'node',
  extensionsToTreatAsEsm: ['.ts'],

  // Multiple test configurations for different test types
  projects: [
    {
      displayName: 'unit',
      testMatch: ['<rootDir>/test/unit/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/unit-setup.ts'],
      testEnvironment: 'node',
      moduleNameMapper: commonModuleNameMapper,
      transform: commonTransform,
      coveragePathIgnorePatterns: ['/node_modules/', '/test/'],
      testPathIgnorePatterns: [
        '/node_modules/',
        '/dist/',
        'test/unit/lib/kubernetes.test.ts',
      ],
    },
    {
      displayName: 'integration',
      testMatch: ['<rootDir>/test/integration/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/integration-setup.ts'],
      testEnvironment: 'node',
      moduleNameMapper: commonModuleNameMapper,
      transform: commonTransform,
      testPathIgnorePatterns: [
        '/node_modules/',
        '/dist/',
        'test/integration/kubernetes-fast-fail.test.ts',
        'test/integration/error-guidance-propagation.test.ts',
        'test/integration/single-app-flow.test.ts',
        'test/integration/orchestrator-routing.test.ts', // ES module loading issues with @kubernetes/client-node
      ],
    },
    {
      displayName: 'e2e',
      testMatch: ['<rootDir>/test/e2e/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/e2e-setup.ts'],
      testEnvironment: 'node',
      moduleNameMapper: commonModuleNameMapper,
      transform: commonTransform,
      maxWorkers: 1,
    },
  ],

  // Transform ESM packages
  transformIgnorePatterns: ['node_modules/(?!(@kubernetes/client-node)/)'],

  // Performance optimizations
  maxWorkers: '50%', // Use half of available CPU cores
  cache: true,
  cacheDirectory: '<rootDir>/node_modules/.cache/jest',

  // Coverage configuration
  collectCoverageFrom: [
    'src/**/*.ts',
    '!src/**/*.d.ts',
    '!src/**/*.test.ts',
    '!src/**/*.spec.ts',
    '!src/**/index.ts',
  ],
  coverageDirectory: 'coverage',
  coverageReporters: ['text', 'lcov', 'html', 'json-summary'],
  coverageThreshold: {
    global: {
      branches: 7,
      functions: 18,
      lines: 8,
      statements: 9,
    },
    './src/mcp/': {
      branches: 14,
      functions: 22,
      lines: 20,
      statements: 19,
    },
    './src/tools/': {
      branches: 51,
      functions: 55,
      lines: 62,
      statements: 62,
    },
    './src/workflows/': {
      branches: 0,
      functions: 0,
      lines: 0,
      statements: 0,
    },
    './src/lib/': {
      branches: 22,
      functions: 41,
      lines: 39,
      statements: 39,
    },
  },

  // File extensions and test configuration
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json', 'node'],
  roots: ['<rootDir>/src', '<rootDir>/test'],
  testPathIgnorePatterns: [
    '/node_modules/',
    '/dist/',
    'test/unit/lib/kubernetes.test.ts',
    'test/integration/kubernetes-fast-fail.test.ts', // ES module loading issues with @kubernetes/client-node
    'test/integration/error-guidance-propagation.test.ts', // Imports kubernetes client
    'test/integration/single-app-flow.test.ts', // Imports kubernetes client
    'test/integration/multi-module-flow.test.ts', // Imports kubernetes client
  ],

  // Timeout handling for different test types
  testTimeout: 30000, // Default 30s

  // Better error reporting
  verbose: false, // Reduce noise for CI
  silent: false,

  // Fail fast for development
  bail: false, // Continue running tests to get full picture

  // Global setup and teardown
  globalSetup: '<rootDir>/test/__support__/setup/global-setup.ts',
  globalTeardown: '<rootDir>/test/__support__/setup/global-teardown.ts',

  // Setup files
  setupFilesAfterEnv: ['<rootDir>/test/setup.ts'],
};
