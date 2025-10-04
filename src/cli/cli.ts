#!/usr/bin/env node
/**
 * Containerization Assist MCP CLI
 * Command-line interface for the Containerization Assist MCP Server
 */

import { program } from 'commander';
import { createApp } from '@/app';
import { config, logConfigSummaryIfDev } from '@/config/index';
import { createLogger } from '@/lib/logger';
import { exit, argv, env, cwd } from 'node:process';
import { execSync } from 'node:child_process';
import { readFileSync, statSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { extractErrorMessage } from '@/lib/error-utils';
import { autoDetectDockerSocket } from '@/infra/docker/client';
import { createInspectToolsCommand } from './commands/inspect-tools';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const packageJsonPath = __dirname.includes('dist')
  ? join(__dirname, '../../../package.json') // dist/src/cli/ -> root
  : join(__dirname, '../../package.json'); // src/cli/ -> root
const packageJson = JSON.parse(readFileSync(packageJsonPath, 'utf-8'));

let logger: ReturnType<typeof createLogger> | null = null;
function getLogger(): ReturnType<typeof createLogger> {
  if (!logger) {
    logger = createLogger({ name: 'cli' });
  }
  return logger;
}

program
  .name('containerization-assist-mcp')
  .description('MCP server for AI-powered containerization workflows')
  .version(packageJson.version)
  .argument('[command]', 'command to run (start, inspect-tools)', 'start')
  .option('--config <path>', 'path to configuration file (.env)')
  .option('--log-level <level>', 'logging level: debug, info, warn, error (default: info)', 'info')
  .option('--workspace <path>', 'workspace directory path (default: current directory)', cwd())
  .option('--dev', 'enable development mode with debug logging')
  .option('--validate', 'validate configuration and exit')
  .option('--list-tools', 'list all registered MCP tools and exit')
  .option('--health-check', 'perform system health check and exit')
  .option('--docker-socket <path>', 'Docker socket path (default: platform-specific)', '')
  .option(
    '--k8s-namespace <namespace>',
    'default Kubernetes namespace (default: default)',
    'default',
  )
  .addHelpText(
    'after',
    `

Examples:
  $ containerization-assist-mcp                           Start server with stdio transport
  $ containerization-assist-mcp --dev --log-level debug  Start in development mode with debug logs
  $ containerization-assist-mcp --list-tools             Show all available MCP tools
  $ containerization-assist-mcp --health-check           Check system dependencies
  $ containerization-assist-mcp --validate               Validate configuration

MCP Tools Available:
  • Analysis: analyze-repo, resolve-base-images
  • Build: generate-dockerfile, build-image, scan-image
  • Registry: tag-image, push-image
  • Deploy: generate-k8s-manifests, prepare-cluster, deploy, verify-deploy
  • Additional: ops, inspect-session, fix-dockerfile

For detailed documentation, see: docs/README.md
For examples and tutorials, see: docs/examples/

Environment Variables:
  LOG_LEVEL                 Logging level (debug, info, warn, error)
  WORKSPACE_DIR            Working directory for operations
  DOCKER_SOCKET            Docker daemon socket path
  K8S_NAMESPACE            Default Kubernetes namespace
  NODE_ENV                 Environment (development, production)
`,
  );

program.parse(argv);

const options = program.opts();
const command = program.args[0] ?? 'start';

// Enhanced Docker socket validation
function validateDockerSocket(options: any): { dockerSocket: string; warnings: string[] } {
  const warnings: string[] = [];
  let dockerSocket = '';
  const defaultDockerSocket = autoDetectDockerSocket();

  // Priority order: CLI option -> Environment variable -> Default
  if (options.dockerSocket) {
    dockerSocket = options.dockerSocket;
  } else if (process.env.DOCKER_SOCKET) {
    dockerSocket = process.env.DOCKER_SOCKET;
  } else {
    dockerSocket = defaultDockerSocket;
  }

  // Validate the selected socket
  try {
    // Handle Windows named pipes specially - they can't be stat()'d
    if (dockerSocket.includes('pipe')) {
      // For Windows named pipes, assume they're valid and let Docker client handle validation
      if (!process.env.MCP_MODE && !process.env.MCP_QUIET) {
        console.error(`✅ Using Docker named pipe: ${dockerSocket}`);
      }
      return { dockerSocket, warnings };
    }

    // For Unix sockets and other paths, check if they exist and are valid
    const stat = statSync(dockerSocket);
    if (!stat.isSocket()) {
      warnings.push(`${dockerSocket} exists but is not a socket`);
      return {
        dockerSocket: '',
        warnings: [
          ...warnings,
          'No valid Docker socket found',
          'Docker operations require a valid Docker connection',
          'Consider: 1) Starting Docker Desktop, 2) Specifying --docker-socket <path>',
        ],
      };
    }

    // Only log when not in pure MCP mode or quiet mode
    if (!process.env.MCP_MODE && !process.env.MCP_QUIET) {
      console.error(`✅ Using Docker socket: ${dockerSocket}`);
    }
  } catch (error) {
    const errorMsg = extractErrorMessage(error);
    warnings.push(`Cannot access Docker socket: ${dockerSocket} - ${errorMsg}`);
    return {
      dockerSocket: '',
      warnings: [
        ...warnings,
        'No valid Docker socket found',
        'Docker operations require a valid Docker connection',
        'Consider: 1) Starting Docker Desktop, 2) Specifying --docker-socket <path>',
      ],
    };
  }

  return { dockerSocket, warnings };
}

function provideContextualGuidance(error: Error, options: any): void {
  console.error(`\n🔍 Error: ${error.message}`);

  // Docker-related guidance
  if (error.message.includes('Docker') || error.message.includes('ENOENT')) {
    console.error('\n💡 Docker-related issue detected:');
    console.error('  • Ensure Docker Desktop/Engine is running');
    console.error('  • Verify Docker socket access permissions');
    console.error('  • Check Docker socket path with: docker context ls');
    console.error('  • Test Docker connection: docker version');
    console.error('  • Check Docker daemon is running');
    console.error('  • Specify custom socket: --docker-socket <path>');
  }

  // Permission guidance
  if (error.message.includes('permission') || error.message.includes('EACCES')) {
    console.error('\n💡 Permission issue detected:');
    console.error('  • Check file/directory permissions: ls -la');
    console.error('  • Verify workspace is accessible: --workspace <path>');
    console.error('  • Ensure Docker socket permissions (add user to docker group)');
    console.error('  • Consider running with appropriate permissions');
  }

  // Configuration guidance
  if (error.message.includes('config') || error.message.includes('Config')) {
    console.error('\n💡 Configuration issue:');
    console.error('  • Copy .env.example to .env: cp .env.example .env');
    console.error('  • Validate configuration: --validate');
    console.error('  • Check config file exists: --config <path>');
    console.error('  • Review configuration docs: docs/CONFIGURATION.md');
  }

  console.error('\n🛠️ General troubleshooting steps:');
  console.error('  1. Run health check: containerization-assist-mcp --health-check');
  console.error('  2. Validate config: containerization-assist-mcp --validate');
  console.error('  3. Check Docker: docker version');
  console.error('  4. Enable debug logging: --log-level debug --dev');
  console.error('  5. Check system requirements: docs/REQUIREMENTS.md');
  console.error('  6. Review troubleshooting guide: docs/TROUBLESHOOTING.md');

  if (options.dev && error.stack) {
    console.error(`\n📍 Stack trace (dev mode):`);
    console.error(error.stack);
  } else if (!options.dev) {
    console.error('\n💡 For detailed error information, use --dev flag');
  }
}

// Validation function for CLI options
function validateOptions(opts: any): { valid: boolean; errors: string[] } {
  const errors: string[] = [];

  const validLogLevels = ['debug', 'info', 'warn', 'error'];
  if (opts.logLevel && !validLogLevels.includes(opts.logLevel)) {
    errors.push(`Invalid log level: ${opts.logLevel}. Valid options: ${validLogLevels.join(', ')}`);
  }

  // Validate workspace directory exists
  if (opts.workspace) {
    try {
      const stat = statSync(opts.workspace);
      if (!stat.isDirectory()) {
        errors.push(`Workspace path is not a directory: ${opts.workspace}`);
      }
    } catch (error) {
      const errorMsg = extractErrorMessage(error);
      if (errorMsg.includes('ENOENT')) {
        errors.push(`Workspace directory does not exist: ${opts.workspace}`);
      } else if (errorMsg.includes('EACCES')) {
        errors.push(`Permission denied accessing workspace: ${opts.workspace}`);
      } else {
        errors.push(`Cannot access workspace directory: ${opts.workspace} (${errorMsg})`);
      }
    }
  }

  // Enhanced Docker socket validation
  const dockerValidation = validateDockerSocket(opts);
  opts.dockerSocket = dockerValidation.dockerSocket;

  // Add warnings as non-fatal errors for user awareness
  if (dockerValidation.warnings.length > 0) {
    dockerValidation.warnings.forEach((warning) => {
      if (warning.includes('No valid Docker socket')) {
        errors.push(warning);
      } else if (!process.env.MCP_MODE) {
        console.error(`⚠️  ${warning}`);
      }
    });
  }

  // Validate config file exists if specified
  if (opts.config) {
    try {
      statSync(opts.config);
    } catch (error) {
      const errorMsg = extractErrorMessage(error);
      errors.push(`Configuration file not found: ${opts.config} - ${errorMsg}`);
    }
  }

  return { valid: errors.length === 0, errors };
}

async function main(): Promise<void> {
  try {
    // Handle different commands
    if (command === 'inspect-tools') {
      // Parse subcommand for inspect-tools
      const inspectCmd = createInspectToolsCommand();
      const subArgs = argv.slice(3); // Skip 'node', script name, and 'inspect-tools'
      await inspectCmd.parseAsync(['node', 'inspect-tools', ...subArgs], { from: 'node' });
      return;
    }

    // Handle the 'start' command (default behavior)
    if (command !== 'start') {
      console.error(`❌ Unknown command: ${command}`);
      console.error('Available commands: start, inspect-tools');
      console.error('\nUse --help for usage information');
      exit(1);
    }

    // Validate CLI options
    const validation = validateOptions(options);
    if (!validation.valid) {
      console.error('❌ Configuration errors:');
      validation.errors.forEach((error) => console.error(`  • ${error}`));
      console.error('\nUse --help for usage information');
      exit(1);
    }

    // Set environment variables based on CLI options
    if (options.logLevel) env.LOG_LEVEL = options.logLevel;
    if (options.workspace) env.WORKSPACE_DIR = options.workspace;
    if (options.dockerSocket) process.env.DOCKER_SOCKET = options.dockerSocket;
    if (options.k8sNamespace) process.env.K8S_NAMESPACE = options.k8sNamespace;
    if (options.dev) process.env.NODE_ENV = 'development';

    // Log configuration summary in development mode
    logConfigSummaryIfDev();

    if (options.validate) {
      console.error('🔍 Validating Containerization Assist MCP configuration...\n');
      console.error('📋 Configuration Summary:');
      console.error(`  • Log Level: ${config.server.logLevel}`);
      console.error(`  • Workspace: ${config.workspace?.workspaceDir ?? process.cwd()}`);
      console.error(`  • Docker Socket: ${process.env.DOCKER_SOCKET ?? '/var/run/docker.sock'}`);
      console.error(`  • K8s Namespace: ${process.env.K8S_NAMESPACE ?? 'default'}`);
      console.error(`  • Environment: ${process.env.NODE_ENV ?? 'production'}`);

      // Test Docker connection
      {
        console.error('\n🐳 Testing Docker connection...');
        try {
          execSync('docker version', { stdio: 'pipe' });
          console.error('  ✅ Docker connection successful');
        } catch (error) {
          const errorMsg = extractErrorMessage(error);
          console.error(`  ⚠️  Docker connection failed - ensure Docker is running: ${errorMsg}`);
        }
      }

      // Test Kubernetes connection
      console.error('\n☸️  Testing Kubernetes connection...');
      try {
        execSync('kubectl version --client=true', { stdio: 'pipe' });
        console.error('  ✅ Kubernetes client available');
      } catch (error) {
        const errorMsg = extractErrorMessage(error);
        console.error(`  ⚠️  Kubernetes client not found - kubectl not in PATH: ${errorMsg}`);
      }

      getLogger().info('Configuration validation completed');
      console.error('\n✅ Configuration validation complete!');
      console.error('\nNext steps:');
      console.error('  • Start server: containerization-assist-mcp');
      console.error('  • List tools: containerization-assist-mcp --list-tools');
      console.error('  • Health check: containerization-assist-mcp --health-check');
      process.exit(0);
    }

    // Set MCP mode to redirect logs to stderr
    process.env.MCP_MODE = 'true';

    // Create the application
    const app = createApp({
      logger: getLogger(),
      policyPath: options.config || 'config/policy.yaml',
      policyEnvironment: options.dev ? 'development' : 'production',
    });

    if (options.listTools) {
      getLogger().info('Listing available tools');

      const tools = app.listTools();

      console.error('\n🛠️  Available MCP Tools:');
      console.error('═'.repeat(60));

      console.error('\n📦 Containerization Tools:');
      tools.forEach((tool: { name: string; description: string }) => {
        console.error(`  • ${tool.name.padEnd(30)} - ${tool.description}`);
      });

      console.error('\n📊 Summary:');
      console.error(`  • Total tools: ${tools.length}`);

      process.exit(0);
    }

    if (options.healthCheck) {
      getLogger().info('Performing health check');

      const health = app.healthCheck();

      console.error('🏥 Health Check Results');
      console.error('═'.repeat(40));
      console.error(`Status: ✅ ${health.status}`);
      console.error('\nServices:');
      console.error(`  ✅ MCP Server: ready`);
      console.error(`  📦 Tools loaded: ${health.tools}`);

      process.exit(0);
    }

    const transportConfig = {
      transport: 'stdio' as const,
    };

    // Use shared startup logging
    const { logStartup, logStartupSuccess, installShutdownHandlers } = await import(
      '@/lib/runtime-logging'
    );

    const health = app.healthCheck();
    logStartup(
      {
        appName: 'containerization-assist-mcp',
        version: packageJson.version,
        workspace: config.workspace?.workspaceDir || process.cwd(),
        logLevel: config.server.logLevel,
        transport: transportConfig,
        devMode: options.dev,
        toolCount: health.tools,
      },
      getLogger(),
      !!process.env.MCP_QUIET,
    );

    await app.startServer(transportConfig);

    logStartupSuccess(transportConfig, getLogger(), !!process.env.MCP_QUIET);

    // Install unified shutdown handlers
    installShutdownHandlers(app, getLogger(), !!process.env.MCP_QUIET);
  } catch (error) {
    const { logStartupFailure } = await import('@/lib/runtime-logging');
    logStartupFailure(error as Error, getLogger(), !!process.env.MCP_QUIET);

    if (error instanceof Error) {
      provideContextualGuidance(error, options);
    }

    exit(1);
  }
}

// Uncaught exception and rejection handlers are installed by the unified shutdown handlers

// Run the CLI
void main();
