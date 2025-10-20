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
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { checkDockerHealth, checkKubernetesHealth } from '@/lib/health-checks';
import { validateDockerSocket } from '@/infra/docker/socket-validation';
import { createInspectToolsCommand } from './commands/inspect-tools';
import { provideContextualGuidance } from './guidance';
import { validateOptions } from './validation';
import { OUTPUTFORMAT } from '@/mcp/mcp-server';
import {
  logStartup,
  logStartupSuccess,
  installShutdownHandlers,
  logStartupFailure,
} from '@/lib/runtime-logging';

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

MCP Tools Available (13 total):
  • Analysis: analyze-repo
  • Dockerfile: generate-dockerfile, validate-dockerfile, fix-dockerfile
  • Image: build-image, scan-image, tag-image, push-image
  • Kubernetes: generate-k8s-manifests, prepare-cluster, deploy, verify-deploy
  • Utilities: ops

For detailed documentation, see: README.md
For examples and tutorials, see: docs/examples/

Environment Variables:
  LOG_LEVEL                                    Logging level (debug, info, warn, error)
  WORKSPACE_DIR                                Working directory for operations
  DOCKER_SOCKET                                Docker daemon socket path
  K8S_NAMESPACE                                Default Kubernetes namespace
  CONTAINERIZATION_ASSIST_POLICY_PATH          Policy file path (overridden by --config)
  NODE_ENV                                     Environment (development, production)
`,
  );

program.parse(argv);

const options = program.opts();
const command = program.args[0] ?? 'start';

/**
 * Resolve policy configuration with priority:
 * 1. CLI flag (highest priority)
 * 2. Environment variable
 * 3. Default value (undefined = auto-discover)
 */
function resolvePolicyConfig(options: { config?: string }): { policyPath?: string } {
  // Policy path: --config flag > env var > undefined (use defaults)
  const policyPath = options.config || process.env.CONTAINERIZATION_ASSIST_POLICY_PATH;

  // Only include policyPath if it has a value (for exactOptionalPropertyTypes)
  if (policyPath) {
    return { policyPath };
  }
  return {};
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
    const dockerValidation = validateDockerSocket(options);
    const validation = validateOptions(options, dockerValidation);
    if (!validation.valid) {
      console.error('❌ Configuration errors:');
      validation.errors.forEach((error: string) => console.error(`  • ${error}`));
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

      // Display policy configuration
      const policyConfig = resolvePolicyConfig(options);
      console.error(`  • Policy Path: ${policyConfig.policyPath ?? 'auto-discover'}`);

      // Test Docker and Kubernetes connections
      console.error('\n🔍 Checking dependencies...');
      const dockerStatus = await checkDockerHealth(getLogger());
      const k8sStatus = await checkKubernetesHealth(getLogger());

      console.error(
        dockerStatus.available
          ? `  ✅ Docker: ${dockerStatus.version}`
          : `  ⚠️  Docker: ${dockerStatus.error}`,
      );

      console.error(
        k8sStatus.available
          ? `  ✅ Kubernetes: ${k8sStatus.version || 'connected'}`
          : `  ⚠️  Kubernetes: ${k8sStatus.error}`,
      );

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

    // Resolve policy configuration from CLI flags and environment variables
    const policyConfig = resolvePolicyConfig(options);

    // Create the application
    const app = createApp({
      logger: getLogger(),
      ...policyConfig,
      outputFormat: OUTPUTFORMAT.MARKDOWN,
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

      const health = await app.healthCheck();

      console.error('🏥 Health Check Results');
      console.error('═'.repeat(40));
      const statusIcon = health.status === 'healthy' ? '✅' : '⚠️';
      console.error(`Status: ${statusIcon} ${health.status}`);
      console.error('\nServices:');
      console.error(`  ✅ MCP Server: ready`);
      console.error(`  📦 Tools loaded: ${health.tools}`);

      if (health.dependencies) {
        console.error('\nDependencies:');

        if (health.dependencies.docker) {
          const docker = health.dependencies.docker;
          const dockerIcon = docker.available ? '✅' : '❌';
          const dockerInfo = docker.available
            ? docker.version
              ? `v${docker.version}`
              : 'available'
            : docker.error || 'unavailable';
          console.error(`  ${dockerIcon} Docker: ${dockerInfo}`);
        }

        if (health.dependencies.kubernetes) {
          const k8s = health.dependencies.kubernetes;
          const k8sIcon = k8s.available ? '✅' : '❌';
          const k8sInfo = k8s.available ? k8s.version || 'connected' : k8s.error || 'unavailable';
          console.error(`  ${k8sIcon} Kubernetes: ${k8sInfo}`);
        }
      }

      process.exit(health.status === 'healthy' ? 0 : 1);
    }

    const transportConfig = {
      transport: 'stdio' as const,
    };

    // Use shared startup logging
    const health = await app.healthCheck();
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
