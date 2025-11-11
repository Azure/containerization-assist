import { createRequire } from 'module';

const commonModuleNameMapper = {
  // Path aliases from tsconfig
  '^@/(.*)$': '<rootDir>/src/$1',
  '^@types$': '<rootDir>/src/types/index.ts',

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
      transformIgnorePatterns: ['node_modules/(?!(@kubernetes/client-node)/)'],
      collectCoverageFrom: [
        'src/**/*.ts',
        '!src/**/*.d.ts',
        '!src/**/*.test.ts',
        '!src/**/*.spec.ts',
        '!src/**/index.ts',
      ],
      coveragePathIgnorePatterns: ['/node_modules/', '/test/'],
      testPathIgnorePatterns: [
        '/node_modules/',
        '/dist/',
        'test/unit/lib/kubernetes.test.ts',
      ],
      maxWorkers: 1, // Run unit tests serially to prevent worker shutdown issues
    },
    {
      displayName: 'integration',
      testMatch: ['<rootDir>/test/integration/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/integration-setup.ts'],
      testEnvironment: 'node',
      moduleNameMapper: commonModuleNameMapper,
      transform: commonTransform,
      transformIgnorePatterns: ['node_modules/(?!(@kubernetes/client-node)/)'],
      collectCoverageFrom: [
        'src/**/*.ts',
        '!src/**/*.d.ts',
        '!src/**/*.test.ts',
        '!src/**/*.spec.ts',
        '!src/**/index.ts',
      ],
      coveragePathIgnorePatterns: ['/node_modules/', '/test/'],
      testPathIgnorePatterns: [
        '/node_modules/',
        '/dist/',
        // ES module issues resolved - all integration tests enabled
      ],
      workerIdleMemoryLimit: '512MB', // Force worker restart after integration tests
    },
    {
      displayName: 'e2e',
      testMatch: ['<rootDir>/test/e2e/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/e2e-setup.ts'],
      testEnvironment: 'node',
      moduleNameMapper: commonModuleNameMapper,
      transform: commonTransform,
      transformIgnorePatterns: ['node_modules/(?!(@kubernetes/client-node)/)'],
      collectCoverageFrom: [
        'src/**/*.ts',
        '!src/**/*.d.ts',
        '!src/**/*.test.ts',
        '!src/**/*.spec.ts',
        '!src/**/index.ts',
      ],
      coveragePathIgnorePatterns: ['/node_modules/', '/test/'],
      maxWorkers: 1,
    },
    {
      displayName: 'llm-integration',
      testMatch: ['<rootDir>/test/llm-integration/**/*.test.ts'],
      testEnvironment: 'node',
      moduleNameMapper: {
        ...commonModuleNameMapper,
        // Mock kubernetes dependencies for LLM tests
        '@kubernetes/client-node': '<rootDir>/test/__support__/mocks/kubernetes-mock.ts',
      },
      transform: commonTransform,
      collectCoverageFrom: [
        'src/**/*.ts',
        '!src/**/*.d.ts',
        '!src/**/*.test.ts',
        '!src/**/*.spec.ts',
        '!src/**/index.ts',
      ],
      coveragePathIgnorePatterns: ['/node_modules/', '/test/'],
      maxWorkers: 1, // Serial execution for LLM tests to avoid rate limits
      testTimeout: 180000, // 3 minute timeout for LLM interactions
    },
  ],

  // Transform ESM packages
  transformIgnorePatterns: ['node_modules/(?!(@kubernetes/client-node)/)'],

  // Performance optimizations
  maxWorkers: '50%', // Use half of available CPU cores
  workerIdleMemoryLimit: '512MB', // Force worker restart to prevent memory leaks
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
      branches: 48,
      functions: 60,
      lines: 58,
      statements: 58,
    },
    './src/app/': {
      branches: 55,
      functions: 80,
      lines: 74,
      statements: 74,
    },
    './src/infra/': {
      branches: 50,
      functions: 60,
      lines: 55,
      statements: 55,
    },
    './src/tools/': {
      branches: 50,
      functions: 64,
      lines: 63,
      statements: 63,
    },
    './src/lib/': {
      branches: 50,
      functions: 50,
      lines: 60,
      statements: 60,
    },
  },

  // File extensions and test configuration
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json', 'node'],
  roots: ['<rootDir>/src', '<rootDir>/test'],
  testPathIgnorePatterns: [
    '/node_modules/',
    '/dist/',
    'test/unit/lib/kubernetes.test.ts',
    'test/llm-integration/**/*.test.ts', // Exclude LLM integration tests from default runs
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
