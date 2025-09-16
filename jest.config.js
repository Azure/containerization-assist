import { createRequire } from 'module';
const require = createRequire(import.meta.url);

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
      coveragePathIgnorePatterns: ['/node_modules/', '/test/'],
      moduleNameMapper: {
        '^@app/(.*)$': '<rootDir>/src/app/$1',
        '^@config/(.*)$': '<rootDir>/src/config/$1',
        '^@domain/(.*)$': '<rootDir>/src/domain/$1',
        '^@infrastructure/(.*)$': '<rootDir>/src/infrastructure/$1',
        '^@lib/(.*)$': '<rootDir>/src/lib/$1',
        '^@mcp/(.*)$': '<rootDir>/src/mcp/$1',
        '^@tools/(.*)$': '<rootDir>/src/tools/$1',
        '^@workflows/(.*)$': '<rootDir>/src/workflows/$1',
        '^@resources/(.*)$': '<rootDir>/src/resources/$1',
        '^@prompts/(.*)$': '<rootDir>/src/prompts/$1',
        '^@knowledge/(.*)$': '<rootDir>/src/knowledge/$1',
        '^@services/(.*)$': '<rootDir>/src/services/$1',
        '^@exports/(.*)$': '<rootDir>/src/exports/$1',
        '^@types$': '<rootDir>/src/types',
        // Missing @/* patterns that match tsconfig.json
        '^@/(.*)$': '<rootDir>/src/$1',
        '^@/container$': '<rootDir>/src/container',
        '^@/validation$': '<rootDir>/src/validation',
        '^@/types$': '<rootDir>/src/types',
        '^(\\.{1,2}/.*)\\.js$': '$1',
        // Test support mappings
        '^@test/fixtures/(.*)$': '<rootDir>/test/__support__/fixtures/$1',
        '^@test/utilities/(.*)$': '<rootDir>/test/__support__/utilities/$1',
        '^@test/mocks/(.*)$': '<rootDir>/test/__support__/mocks/$1',
      },
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
              isolatedModules: true
            },
          },
        ],
      },
    },
    {
      displayName: 'integration',
      testMatch: ['<rootDir>/test/integration/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/integration-setup.ts'],
      testEnvironment: 'node',
      moduleNameMapper: {
        '^@app/(.*)$': '<rootDir>/src/app/$1',
        '^@config/(.*)$': '<rootDir>/src/config/$1',
        '^@domain/(.*)$': '<rootDir>/src/domain/$1',
        '^@infrastructure/(.*)$': '<rootDir>/src/infrastructure/$1',
        '^@lib/(.*)$': '<rootDir>/src/lib/$1',
        '^@mcp/(.*)$': '<rootDir>/src/mcp/$1',
        '^@tools/(.*)$': '<rootDir>/src/tools/$1',
        '^@workflows/(.*)$': '<rootDir>/src/workflows/$1',
        '^@resources/(.*)$': '<rootDir>/src/resources/$1',
        '^@prompts/(.*)$': '<rootDir>/src/prompts/$1',
        '^@knowledge/(.*)$': '<rootDir>/src/knowledge/$1',
        '^@services/(.*)$': '<rootDir>/src/services/$1',
        '^@exports/(.*)$': '<rootDir>/src/exports/$1',
        '^@types$': '<rootDir>/src/types',
        // Missing @/* patterns that match tsconfig.json
        '^@/(.*)$': '<rootDir>/src/$1',
        '^@/container$': '<rootDir>/src/container',
        '^@/validation$': '<rootDir>/src/validation',
        '^@/types$': '<rootDir>/src/types',
        '^(\\.{1,2}/.*)\\.js$': '$1',
        '^@test/fixtures/(.*)$': '<rootDir>/test/__support__/fixtures/$1',
        '^@test/utilities/(.*)$': '<rootDir>/test/__support__/utilities/$1',
        '^@test/mocks/(.*)$': '<rootDir>/test/__support__/mocks/$1',
      },
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
              isolatedModules: true
            },
          },
        ],
      },
    },
    {
      displayName: 'e2e',
      testMatch: ['<rootDir>/test/e2e/**/*.test.ts'],
      setupFilesAfterEnv: ['<rootDir>/test/__support__/setup/e2e-setup.ts'],
      testEnvironment: 'node',
      maxWorkers: 1,
      moduleNameMapper: {
        '^@app/(.*)$': '<rootDir>/src/app/$1',
        '^@config/(.*)$': '<rootDir>/src/config/$1',
        '^@domain/(.*)$': '<rootDir>/src/domain/$1',
        '^@infrastructure/(.*)$': '<rootDir>/src/infrastructure/$1',
        '^@lib/(.*)$': '<rootDir>/src/lib/$1',
        '^@mcp/(.*)$': '<rootDir>/src/mcp/$1',
        '^@tools/(.*)$': '<rootDir>/src/tools/$1',
        '^@workflows/(.*)$': '<rootDir>/src/workflows/$1',
        '^@resources/(.*)$': '<rootDir>/src/resources/$1',
        '^@prompts/(.*)$': '<rootDir>/src/prompts/$1',
        '^@knowledge/(.*)$': '<rootDir>/src/knowledge/$1',
        '^@services/(.*)$': '<rootDir>/src/services/$1',
        '^@exports/(.*)$': '<rootDir>/src/exports/$1',
        '^@types$': '<rootDir>/src/types',
        // Missing @/* patterns that match tsconfig.json
        '^@/(.*)$': '<rootDir>/src/$1',
        '^@/container$': '<rootDir>/src/container',
        '^@/validation$': '<rootDir>/src/validation',
        '^@/types$': '<rootDir>/src/types',
        '^(\\.{1,2}/.*)\\.js$': '$1',
        '^@test/fixtures/(.*)$': '<rootDir>/test/__support__/fixtures/$1',
        '^@test/utilities/(.*)$': '<rootDir>/test/__support__/utilities/$1',
        '^@test/mocks/(.*)$': '<rootDir>/test/__support__/mocks/$1',
      },
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
              isolatedModules: true
            },
          },
        ],
      },
    },
  ],

  // Global configuration
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
          isolatedModules: true
        },
      },
    ],
  },

  // Transform ESM packages
  transformIgnorePatterns: [
    'node_modules/(?!(@kubernetes/client-node)/)'
  ],
  // Performance optimizations
  maxWorkers: '50%',  // Use half of available CPU cores
  cache: true,
  cacheDirectory: '<rootDir>/node_modules/.cache/jest',
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
      statements: 9
    },
    './src/mcp/': {
      branches: 14,
      functions: 22,
      lines: 20,
      statements: 19
    },
    './src/tools/': {
      branches: 51,
      functions: 55,
      lines: 62,
      statements: 62
    },
    './src/workflows/': {
      branches: 0,
      functions: 0,
      lines: 0,
      statements: 0
    },
    './src/lib/': {
      branches: 22,
      functions: 41,
      lines: 39,
      statements: 39
    }
  },
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json', 'node'],
  moduleNameMapper: {
    // Path aliases from tsconfig
    '^@app/(.*)$': '<rootDir>/src/app/$1',
    '^@config/(.*)$': '<rootDir>/src/config/$1',
    '^@domain/(.*)$': '<rootDir>/src/domain/$1',
    '^@lib/(.*)$': '<rootDir>/src/lib/$1',
    '^@mcp/(.*)$': '<rootDir>/src/mcp/$1',
    '^@tools/(.*)$': '<rootDir>/src/tools/$1',
    '^@workflows/(.*)$': '<rootDir>/src/workflows/$1',
    '^@resources/(.*)$': '<rootDir>/src/resources/$1',
    '^@prompts/(.*)$': '<rootDir>/src/prompts/$1',
    '^@knowledge/(.*)$': '<rootDir>/src/knowledge/$1',
    '^@services/(.*)$': '<rootDir>/src/services/$1',
    '^@exports/(.*)$': '<rootDir>/src/exports/$1',
    '^@types$': '<rootDir>/src/types',
    // Missing @/* patterns that match tsconfig.json
    '^@/(.*)$': '<rootDir>/src/$1',

    // Handle .js imports and map them to .ts
    '^(\\.{1,2}/.*)\\.js$': '$1',

    // ESM modules that need special handling
    '@kubernetes/client-node': '@kubernetes/client-node',

    // Core types mapping from different locations
    '^\\.\\./core/types\\.js$': '<rootDir>/src/domain/types.ts',
    '^\\./core/types\\.js$': '<rootDir>/src/domain/types.ts',
    '^\\.\\./\\.\\./core/types\\.js$': '<rootDir>/src/domain/types.ts',

    // Test support mappings
    '^@test/fixtures/(.*)$': '<rootDir>/test/__support__/fixtures/$1',
    '^@test/utilities/(.*)$': '<rootDir>/test/__support__/utilities/$1',
    '^@test/mocks/(.*)$': '<rootDir>/test/__support__/mocks/$1',

    // Legacy test mappings (for backward compatibility during migration)
    '^@fixtures/(.*)$': '<rootDir>/test/__support__/fixtures/$1',
    '^@helpers/(.*)$': '<rootDir>/test/__support__/utilities/$1',
    '^@mocks/(.*)$': '<rootDir>/test/__support__/mocks/$1',

    // Handle specific .js imports and map them to .ts
    // Infrastructure logger fix for test setup (exact path from test/setup.ts)
    '^\\.\\.\/src\/infrastructure\/logger\\.js$': '<rootDir>/src/infrastructure/logger.ts',
    '^\\.\\.\/src\/infrastructure\/core\/logger\\.js$': '<rootDir>/src/infrastructure/logger.ts',
    '^\\.\\.\/\\.\\.\/infrastructure\/core\/logger\\.js$': '<rootDir>/src/infrastructure/logger.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/infrastructure\/core\/logger\\.js$': '<rootDir>/src/infrastructure/logger.ts',

    // Relative imports from domain/types/errors
    '^\\.\\.\/errors\/index\\.js$': '<rootDir>/src/domain/types/errors/index.ts',
    '^\\.\\.\/\\.\\.\/errors\/index\\.js$': '<rootDir>/src/domain/types/errors/index.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/errors\/index\\.js$': '<rootDir>/src/domain/types/errors/index.ts',

    // Relative imports from src/errors - as seen from infrastructure directory
    '^\\.\\./errors/index\\.js$': '<rootDir>/src/errors/index.ts',
    '^\\.\\.\/\\.\\./errors/index\\.js$': '<rootDir>/src/errors/index.ts',
    '^\\.\\.\/\\.\\.\/\\.\\./errors/index\\.js$': '<rootDir>/src/errors/index.ts',

    // Also handle src/errors/index imports from test directories
    '^\\.\\.\/\\.\\.\/\\.\\.\/src\/errors\/index$': '<rootDir>/src/errors/index.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/src\/errors\/index\\.js$': '<rootDir>/src/errors/index.ts',

    // Contract type errors
    '^\\.\\.\/contracts\/types\/errors\\.js$': '<rootDir>/src/contracts/types/errors.ts',
    '^\\.\\.\/\\.\\.\/contracts\/types\/errors\\.js$': '<rootDir>/src/contracts/types/errors.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/contracts\/types\/errors\\.js$': '<rootDir>/src/contracts/types/errors.ts',

    // Infrastructure module mappings
    '^\\.\\.\/command-executor\\.js$': '<rootDir>/src/infrastructure/command-executor.ts',
    '^\\.\\.\/\\.\\.\/command-executor\\.js$': '<rootDir>/src/infrastructure/command-executor.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/src\/infrastructure\/command-executor$': '<rootDir>/src/infrastructure/command-executor.ts',

    // Scanner mappings (from test/unit/infrastructure directories)
    '^\\.\\.\/\\.\\.\/\\.\\.\/src\/infrastructure\/scanners\/trivy-scanner$': '<rootDir>/src/infrastructure/scanners/trivy-scanner.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/src\/infrastructure\/scanners\/trivy-scanner\\.js$': '<rootDir>/src/infrastructure/scanners/trivy-scanner.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/\\.\\.\/src\/infrastructure\/scanners\/trivy-scanner$': '<rootDir>/src/infrastructure/scanners/trivy-scanner.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/\\.\\.\/src\/infrastructure\/scanners\/trivy-scanner\\.js$': '<rootDir>/src/infrastructure/scanners/trivy-scanner.ts',

    // Docker and Kubernetes client mappings
    '^\\.\\.\/\\.\\.\/\\.\\.\/src\/infrastructure\/docker-client$': '<rootDir>/src/infrastructure/docker-client.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/src\/infrastructure\/kubernetes-client$': '<rootDir>/src/infrastructure/kubernetes-client.ts',

    // Domain type imports
    '^\\.\\.\/\\.\\.\/domain\/types\/result\\.js$': '<rootDir>/src/domain/types/result.ts',

    // Helper imports
    '^\\.\/helper\\.js$': './helper.ts',
    '^\\.\\.\/helper\\.js$': '../helper.ts',

    // MCP resources - map .js imports to .ts files (specific patterns)
    '^\\.\\.\/\\.\\.\/src\/mcp\/resources\/(.*)\\.js$': '<rootDir>/src/mcp/resources/$1.ts',
    '^\\.\\.\/\\.\\.\/src\/mcp\/events\/(.*)\\.js$': '<rootDir>/src/mcp/events/$1.ts',
    '^\\.\\.\/\\.\\.\/\\.\\.\/src\/workflows\/(.*)\\.js$': '<rootDir>/src/workflows/$1.ts',
    '^\\.\\.\/\\.\\.\/types\/core\\.js$': '<rootDir>/src/types/core.ts',
    '^\\.\/types\\.js$': '<rootDir>/src/mcp/resources/types.ts',
    '^\\.\/uri-schemes\\.js$': '<rootDir>/src/mcp/resources/uri-schemes.ts',
    '^\\.\/cache\\.js$': '<rootDir>/src/mcp/resources/cache.ts',
  },
  roots: ['<rootDir>/src', '<rootDir>/test'],
  testPathIgnorePatterns: ['/node_modules/', '/dist/', 'test/unit/lib/kubernetes.test.ts'],

  // Timeout handling for different test types
  testTimeout: 30000,  // Default 30s

  // Better error reporting
  verbose: false,  // Reduce noise for CI
  silent: false,

  // Fail fast for development
  bail: false,  // Continue running tests to get full picture

  // Global setup and teardown  
  globalSetup: '<rootDir>/test/__support__/setup/global-setup.ts',
  globalTeardown: '<rootDir>/test/__support__/setup/global-teardown.ts',

  // Setup files
  setupFilesAfterEnv: ['<rootDir>/test/setup.ts'],
};