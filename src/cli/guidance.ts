/**
 * Contextual guidance module for CLI error handling
 * Provides helpful troubleshooting steps based on error types
 */

export interface GuidanceOptions {
  dev?: boolean;
}

/**
 * Error categories for contextual guidance
 */
const ErrorCategory = {
  Docker: 'docker',
  Permission: 'permission',
  Configuration: 'configuration',
} as const;
type ErrorCategory = (typeof ErrorCategory)[keyof typeof ErrorCategory];

/**
 * Guidance messages organized by category
 */
const GUIDANCE_MESSAGES = {
  [ErrorCategory.Docker]: {
    title: '💡 Docker-related issue detected:',
    steps: [
      'Ensure Docker Desktop/Engine is running',
      'Verify Docker socket access permissions',
      'Check Docker socket path with: docker context ls',
      'Test Docker connection: docker version',
      'Check Docker daemon is running',
      'Specify custom socket: --docker-socket <path>',
    ],
  },
  [ErrorCategory.Permission]: {
    title: '💡 Permission issue detected:',
    steps: [
      'Check file/directory permissions: ls -la',
      'Verify workspace is accessible: --workspace <path>',
      'Ensure Docker socket permissions (add user to docker group)',
      'Consider running with appropriate permissions',
    ],
  },
  [ErrorCategory.Configuration]: {
    title: '💡 Configuration issue:',
    steps: [
      'Copy .env.example to .env: cp .env.example .env',
      'Validate configuration: --validate',
      'Review configuration docs: README.md (Configuration section)',
    ],
  },
};

/**
 * General troubleshooting steps shown for all errors
 */
const GENERAL_TROUBLESHOOTING = [
  'Run health check: containerization-assist-mcp --health-check',
  'Validate config: containerization-assist-mcp --validate',
  'Check Docker: docker version',
  'Enable debug logging: --log-level debug --dev',
  'Check system requirements: README.md (System Requirements section)',
  'Review troubleshooting guide: README.md (Troubleshooting section)',
];

/**
 * Detect error category based on error message
 */
function detectErrorCategory(error: Error): ErrorCategory | null {
  const message = error.message.toLowerCase();

  if (message.includes('docker') || message.includes('enoent')) {
    return ErrorCategory.Docker;
  }

  if (message.includes('permission') || message.includes('eacces')) {
    return ErrorCategory.Permission;
  }

  if (message.includes('config')) {
    return ErrorCategory.Configuration;
  }

  return null;
}

/**
 * Provide contextual guidance based on error type
 * @param error - The error that occurred
 * @param options - CLI options (e.g., dev mode)
 */
export function provideContextualGuidance(error: Error, options: GuidanceOptions = {}): void {
  console.error(`\n🔍 Error: ${error.message}`);

  // Detect and display category-specific guidance
  const category = detectErrorCategory(error);
  if (category && GUIDANCE_MESSAGES[category]) {
    const guidance = GUIDANCE_MESSAGES[category];
    console.error(`\n${guidance.title}`);
    guidance.steps.forEach((step) => console.error(`  • ${step}`));
  }

  // Always show general troubleshooting steps
  console.error('\n🛠️ General troubleshooting steps:');
  GENERAL_TROUBLESHOOTING.forEach((step, index) => {
    console.error(`  ${index + 1}. ${step}`);
  });

  // Show stack trace in dev mode
  if (options.dev && error.stack) {
    console.error(`\n📍 Stack trace (dev mode):`);
    console.error(error.stack);
  } else if (!options.dev) {
    console.error('\n💡 For detailed error information, use --dev flag');
  }
}
