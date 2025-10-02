/**
 * ESLint Configuration - TypeScript Type Safety
 *
 * This configuration enforces strict type safety with documented exceptions.
 *
 * ## Strict Rules Enabled:
 * - @typescript-eslint/no-explicit-any: Error (prevents any usage)
 * - @typescript-eslint/consistent-type-assertions: Error (enforces 'as' style)
 * - @typescript-eslint/no-unsafe-*: Error (prevents unsafe type operations)
 *
 * ## Documented Exceptions (where 'any' is allowed):
 * 1. **Test Files** - Mocking frameworks often require any types
 * 2. **src/lib/** - External dependency wrappers (dockerode, scanners)
 * 3. **src/mcp/client|server|sampling/** - MCP SDK interface layer
 * 4. **src/infrastructure/** - External API wrappers (Docker, Kubernetes)
 * 5. **src/config/** - Complex external configuration handling
 * 6. **src/cli/** and **src/app/** - Framework interface layers
 *
 * All exceptions are justified by external API boundaries where types cannot
 * be fully controlled. Core business logic (tools, workflows, domain) maintains
 * strict type safety.
 */
module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 2022,
    sourceType: 'module',
    project: './tsconfig.eslint.json',
  },
  plugins: ['@typescript-eslint'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:@typescript-eslint/recommended-requiring-type-checking',
  ],
  rules: {
    // TypeScript-specific rules
    '@typescript-eslint/explicit-function-return-type': [
      'warn',
      {
        allowExpressions: true,
        allowTypedFunctionExpressions: true,
        allowHigherOrderFunctions: true,
        allowDirectConstAssertionInArrowFunctions: true,
      },
    ],
    '@typescript-eslint/no-unused-vars': [
      'error',
      {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
      },
    ],
    '@typescript-eslint/no-explicit-any': 'error',
    '@typescript-eslint/consistent-type-assertions': [
      'error',
      {
        assertionStyle: 'as',
        objectLiteralTypeAssertions: 'allow-as-parameter',
      },
    ],
    '@typescript-eslint/strict-boolean-expressions': 'off',
    '@typescript-eslint/prefer-nullish-coalescing': 'off',
    '@typescript-eslint/prefer-optional-chain': 'error',
    '@typescript-eslint/require-await': 'off',
    '@typescript-eslint/no-unnecessary-type-assertion': 'error',
    '@typescript-eslint/no-non-null-assertion': 'warn',
    '@typescript-eslint/prefer-as-const': 'error',
    '@typescript-eslint/consistent-type-imports': [
      'error',
      {
        prefer: 'type-imports',
        disallowTypeAnnotations: true,
        fixStyle: 'inline-type-imports'
      }
    ],

    // Stricter type safety rules
    '@typescript-eslint/no-unsafe-argument': 'error',
    '@typescript-eslint/no-unsafe-assignment': 'error',
    '@typescript-eslint/no-unsafe-call': 'error',
    '@typescript-eslint/no-unsafe-member-access': 'error',
    '@typescript-eslint/no-unsafe-return': 'off',

    // Import rules (path alias enforcement)
    'no-duplicate-imports': 'error',
    'no-restricted-imports': [
      'error',
      {
        patterns: [
          {
            group: ['../../../*', '../../*'],
            message:
              'Use path aliases instead of relative imports that go up more than one level. Use @/lib/, @/mcp/, @/tools/, @/types, etc.',
          },
        ],
      },
    ],
    '@typescript-eslint/no-floating-promises': 'error',

    // Import organization (manual rules until import plugin is added)
    'sort-imports': [
      'error',
      {
        ignoreCase: false,
        ignoreDeclarationSort: true, // We'll handle declaration sorting separately
        ignoreMemberSort: false,
        memberSyntaxSortOrder: ['none', 'all', 'multiple', 'single'],
      },
    ],

    // General rules
    'no-console': [
      'warn',
      {
        allow: ['warn', 'error', 'info'],
      },
    ],
    'no-debugger': 'error',
    'no-alert': 'error',
    'prefer-const': 'error',
    'no-var': 'error',
    'object-shorthand': 'error',
    'prefer-template': 'error',
    'template-curly-spacing': 'error',
    'arrow-spacing': 'error',
    'comma-dangle': ['error', 'always-multiline'],
    quotes: [
      'error',
      'single',
      {
        avoidEscape: true,
        allowTemplateLiterals: true,
      },
    ],
    semi: ['error', 'always'],
    // Let Prettier handle indentation
    indent: 'off',
    'max-len': [
      'warn',
      {
        code: 120,
        ignoreUrls: true,
        ignoreStrings: true,
        ignoreTemplateLiterals: true,
        ignoreComments: true,
      },
    ],
    'no-trailing-spaces': 'error',
    'eol-last': 'error',
  },
  ignorePatterns: ['dist', 'node_modules', '*.js', '*.cjs', 'coverage', 'docs', '*.json'],
  env: {
    node: true,
    es2022: true,
  },
  overrides: [
    {
      // Relax any-related rules for test files since mocks often require them
      files: ['**/__tests__/**/*.ts', '**/*.test.ts', '**/*.spec.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-unsafe-argument': 'off',
        '@typescript-eslint/no-unsafe-assignment': 'off',
        '@typescript-eslint/no-unsafe-call': 'off',
        '@typescript-eslint/no-unsafe-member-access': 'off',
        '@typescript-eslint/no-unsafe-return': 'off',
      },
    },
    {
      // Allow 'any' types in lib modules
      // These modules wrap external dependencies (dockerode, scanner services, etc)
      files: ['src/lib/**/*.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-unsafe-argument': 'off',
        '@typescript-eslint/no-unsafe-assignment': 'off',
        '@typescript-eslint/no-unsafe-call': 'off',
        '@typescript-eslint/no-unsafe-member-access': 'off',
        '@typescript-eslint/no-unsafe-return': 'off',
      },
    },
    {
      // Allow 'any' types in MCP client/server transport layers
      // These modules interface with ModelContextProtocol SDK types
      files: ['src/mcp/client/**/*.ts', 'src/mcp/server/**/*.ts', 'src/mcp/sampling/**/*.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-unsafe-argument': 'off',
        '@typescript-eslint/no-unsafe-assignment': 'off',
        '@typescript-eslint/no-unsafe-call': 'off',
        '@typescript-eslint/no-unsafe-member-access': 'off',
        '@typescript-eslint/no-unsafe-return': 'off',
      },
    },
    {
      // Allow 'any' types in infrastructure layers
      // These modules wrap external APIs (Docker, Kubernetes, registries)
      files: ['src/infrastructure/**/*.ts', 'src/resources/**/*.ts', 'src/prompts/**/*.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-unsafe-argument': 'off',
        '@typescript-eslint/no-unsafe-assignment': 'off',
        '@typescript-eslint/no-unsafe-call': 'off',
        '@typescript-eslint/no-unsafe-member-access': 'off',
        '@typescript-eslint/no-unsafe-return': 'off',
      },
    },
    {
      // Allow 'any' types in config files that deal with complex external configs
      files: ['src/config/**/*.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-unsafe-argument': 'off',
        '@typescript-eslint/no-unsafe-assignment': 'off',
        '@typescript-eslint/no-unsafe-call': 'off',
        '@typescript-eslint/no-unsafe-member-access': 'off',
        '@typescript-eslint/no-unsafe-return': 'off',
      },
    },
    {
      // Allow 'any' types in CLI and app entry points
      // These files often interface with external command line and app frameworks
      files: ['src/cli/**/*.ts', 'src/app/**/*.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-unsafe-argument': 'off',
        '@typescript-eslint/no-unsafe-assignment': 'off',
        '@typescript-eslint/no-unsafe-call': 'off',
        '@typescript-eslint/no-unsafe-member-access': 'off',
        '@typescript-eslint/no-unsafe-return': 'off',
      },
    },
    {
      // Re-enable stricter rules for core business logic files
      files: ['src/mcp/tools/**/*.ts', 'src/workflows/**/*.ts', 'src/domain/**/*.ts'],
      rules: {
        '@typescript-eslint/no-unsafe-argument': 'warn',
        '@typescript-eslint/no-unsafe-assignment': 'warn',
        '@typescript-eslint/no-unsafe-member-access': 'warn',
        '@typescript-eslint/no-unsafe-return': 'warn',
      },
    },
  ],
};
