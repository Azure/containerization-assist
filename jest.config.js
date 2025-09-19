import { createRequire } from 'module';
const require = createRequire(import.meta.url);

const commonModuleNameMapper = {
  // Path aliases from tsconfig (simplified)
  '^@/(.*)$': '<rootDir>/src/$1',
  '^@mcp/(.*)$': '<rootDir>/src/mcp/$1',
  '^@tools/(.*)$': '<rootDir>/src/tools/$1',
  '^@lib/(.*)$': '<rootDir>/src/lib/$1',
  '^@services/(.*)$': '<rootDir>/src/services/$1',
  '^@config/(.*)$': '<rootDir>/src/config/$1',
  '^@prompts/(.*)$': '<rootDir>/src/prompts/$1',
  '^@resources/(.*)$': '<rootDir>/src/resources/$1',
  '^@exports/(.*)$': '<rootDir>/src/exports/$1',
  '^@knowledge/(.*)$': '<rootDir>/src/knowledge/$1',
  '^@types$': '<rootDir>/src/types/index.ts',
  '^@validation/(.*)$': '<rootDir>/src/validation/$1',

  // Handle .js imports and map them to .ts
  '^(\\.{1,2}/.*)\\.js$': '$1',

  // Test support mappings
  '^@test/fixtures/(.*)$': '<rootDir>/test/__support__/fixtures/$1',
  '^@test/utilities/(.*)$': '<rootDir>/test/__support__/utilities/$1',
  '^@test/mocks/(.*)$': '<rootDir>/test/__support__/mocks/$1',
};

/** @type {import('jest').Config} */
export default {
  preset: 'ts-jest/presets/default-esm',
  testEnvironment: 'node',
  extensionsToTreatAsEsm: ['.ts'],

  // Simplified projects - just unit and integration
  projects: [
    {
      displayName: 'unit',
      testMatch: ['<rootDir>/test/unit/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/unit-setup.ts'],
      moduleNameMapper: commonModuleNameMapper,
      transform: {
        '^.+\\.tsx?$': [
          'ts-jest',
          {
            useESM: true,
            tsconfig: {
              module: 'ES2022',
              moduleResolution: 'bundler',
              target: 'ES2022',
              allowSyntheticDefaultImports: true,
              esModuleInterop: true,
              isolatedModules: true,
            },
          },
        ],
      },
    },
    {
      displayName: 'integration',
      testMatch: ['<rootDir>/test/integration/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/integration-setup.ts'],
      moduleNameMapper: commonModuleNameMapper,
      transform: {
        '^.+\\.tsx?$': [
          'ts-jest',
          {
            useESM: true,
            tsconfig: {
              module: 'ES2022',
              moduleResolution: 'bundler',
              target: 'ES2022',
              allowSyntheticDefaultImports: true,
              esModuleInterop: true,
              isolatedModules: true,
            },
          },
        ],
      },
    },
  ],

  // Transform ESM packages
  transformIgnorePatterns: ['node_modules/(?!(@kubernetes/client-node)/)'],

  // Coverage configuration
  collectCoverageFrom: [
    'src/**/*.ts',
    '!src/**/*.d.ts',
    '!src/**/*.test.ts',
    '!src/**/*.spec.ts',
    '!src/**/index.ts',
  ],
  coverageDirectory: 'coverage',
  coverageReporters: ['text', 'lcov', 'html'],
  coverageThreshold: {
    global: { branches: 80, functions: 80, lines: 80, statements: 80 }
  },

  // Global setup and teardown
  globalSetup: '<rootDir>/test/__support__/setup/global-setup.ts',
  globalTeardown: '<rootDir>/test/__support__/setup/global-teardown.ts',

  // Performance and timeout settings
  maxWorkers: '50%',
  testTimeout: 30000,
  cache: true,
  cacheDirectory: '<rootDir>/node_modules/.cache/jest',
};