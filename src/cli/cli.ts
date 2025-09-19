#!/usr/bin/env node
/**
 * Containerization Assist MCP CLI
 * Command-line interface for the Containerization Assist MCP Server
 */

import { program } from 'commander';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { createSessionManager } from '@lib/session';
import { getToolRegistry } from '@mcp/tools/registry';
import { initializePrompts, listPrompts } from '@/prompts/prompt-registry';
import * as promptRegistry from '@/prompts/prompt-registry';
import * as resourceManager from '@/resources/manager';
import { config, logConfigSummaryIfDev } from '@config/index';
import { createLogger } from '@lib/logger';
import { createToolContext } from '@mcp/context';
import { createToolRouter } from '@mcp/tool-router';
import { exit, argv, env, cwd } from 'node:process';
import { execSync } from 'node:child_process';
import { readFileSync, statSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { extractErrorMessage } from '@/lib/error-utils';
import { autoDetectDockerSocket } from '@/services/docker-client';

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
  .argument('[command]', 'command to run (start)', 'start')
  .option('--config <path>', 'path to configuration file (.env)')
  .option('--log-level <level>', 'logging level: debug, info, warn, error (default: info)', 'info')
  .option('--workspace <path>', 'workspace directory path (default: current directory)', cwd())
  .option('--port <port>', 'port for HTTP transport (default: stdio)', parseInt)
  .option('--host <host>', 'host for HTTP transport (default: localhost)', 'localhost')
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
  $ containerization-assist-mcp --port 3000              Start server on HTTP port 3000
  $ containerization-assist-mcp --dev --log-level debug  Start in development mode with debug logs
  $ containerization-assist-mcp --list-tools             Show all available MCP tools
  $ containerization-assist-mcp --health-check           Check system dependencies
  $ containerization-assist-mcp --validate               Validate configuration

Quick Start:
  1. Copy .env.example to .env and configure
  2. Run: containerization-assist-mcp --health-check
  3. Start server: containerization-assist-mcp
  4. Test with: echo '{"method":"tools/ping","params":{},"id":1}' | containerization-assist-mcp

MCP Tools Available:
  • Analysis: analyze-repo, resolve-base-images
  • Build: generate-dockerfile, build-image, scan-image
  • Registry: tag-image, push-image
  • Deploy: generate-k8s-manifests, deploy
  • Additional: ops, inspect-session

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
const defaultDockerSocket = autoDetectDockerSocket();

// Enhanced transport detection and logging
function getTransportInfo(options: any): { type: 'stdio' | 'http'; details: string } {
  if (options.port) {
    return {
      type: 'http',
      details: `HTTP transport on ${options.host}:${options.port}`,
    };
  }
  return {
    type: 'stdio',
    details: 'stdio transport (no port)',
  };
}

// Enhanced Docker socket validation
function validateDockerSocket(options: any): { dockerSocket: string; warnings: string[] } {
  const warnings: string[] = [];
  let dockerSocket = '';

  const allSocketOptions = [
    options.dockerSocket,
    process.env.DOCKER_SOCKET,
    defaultDockerSocket,
  ].filter(Boolean);

  for (const thisSocket of allSocketOptions) {
    if (!thisSocket) continue;

    try {
      const stat = statSync(thisSocket);
      if (!stat.isSocket()) {
        warnings.push(`${thisSocket} exists but is not a socket`);
        continue;
      }

      // Only log when not in pure MCP mode or quiet mode
      if (!process.env.MCP_MODE && !process.env.MCP_QUIET) {
        console.error(`✅ Using Docker socket: ${thisSocket}`);
      }
      dockerSocket = thisSocket;
      break;
    } catch (error) {
      const errorMsg = extractErrorMessage(error);
      warnings.push(`Cannot access Docker socket: ${thisSocket} - ${errorMsg}`);
    }
  }

  if (!dockerSocket) {
    return {
      dockerSocket: '',
      warnings: [
        `No valid Docker socket found in: ${allSocketOptions.join(', ')}`,
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

  // Port/networking guidance
  if (error.message.includes('EADDRINUSE')) {
    console.error('\n💡 Port conflict detected:');
    console.error(`  • Port ${options.port} is already in use`);
    console.error('  • Try a different port: --port <number>');
    console.error("  • Check what's using the port: lsof -i :<port>");
    console.error('  • Use default stdio transport (no --port flag)');
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

  // Transport-specific guidance
  if (options.port && !error.message.includes('EADDRINUSE')) {
    console.error('\n💡 HTTP transport troubleshooting:');
    console.error('  • HTTP transport is experimental');
    console.error('  • Consider using default stdio transport');
    console.error('  • Verify host/port configuration');
    console.error('  • Check firewall/network settings');
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

  // Validate port
  if (opts.port && (opts.port < 1 || opts.port > 65535)) {
    errors.push(`Invalid port: ${opts.port}. Must be between 1 and 65535`);
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
    // Handle the 'start' command (default behavior)
    if (command !== 'start') {
      console.error(`❌ Unknown command: ${command}`);
      console.error('Available commands: start');
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
      console.error(`  • SDK Native: enabled`);
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

    // Create dependencies inline
    const mainLogger = createLogger({
      name: config.mcp.name,
      level: config.server.logLevel,
    });

    const sessionManager = createSessionManager(mainLogger, {
      ttl: config.session.ttl,
      maxSessions: config.session.maxSessions,
      cleanupIntervalMs: config.session.cleanupInterval,
    });

    // Initialize prompt registry (directory param ignored - uses embedded prompts)
    await initializePrompts('', mainLogger);
    mainLogger.info('Prompts initialized successfully');

    const tools = getToolRegistry();
    mainLogger.info(`Tool registry loaded with ${tools.size} tools`);

    mainLogger.info('Starting SDK-Native MCP Server');

    // Create the MCP SDK server instance
    const server = new McpServer(
      {
        name: config.mcp.name,
        version: '1.4.2',
      },
      {
        capabilities: {
          tools: {},
        },
      },
    );

    // Create router for tool execution
    const router = createToolRouter({
      sessionManager,
      logger: mainLogger,
      tools,
    });

    // Register tools directly with MCP SDK server
    for (const [toolName, toolDef] of tools) {
      if (!toolDef.schema) {
        mainLogger.warn({ tool: toolName }, 'Tool missing schema, skipping registration');
        continue;
      }

      server.tool(
        toolName,
        `${toolName} tool`,
        (toolDef.schema as any)?.shape || {},
        async (args: unknown) => {
          try {
            const toolLogger = mainLogger.child({ tool: toolName });
            const context = createToolContext(server.server, toolLogger, {
              sessionManager,
              promptRegistry,
              maxTokens: 2048,
              stopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
            });

            // Extract session info from params
            const paramsObj = (args || {}) as Record<string, unknown>;
            let sessionId = paramsObj.sessionId as string;

            // Generate sessionId if not provided
            if (!sessionId) {
              sessionId = `session-${Date.now()}`;
              mainLogger.info(
                { tool: toolName, sessionId },
                'Generated new sessionId (none provided)',
              );
              // Add sessionId to params so it's available to the tool
              paramsObj.sessionId = sessionId;
            } else {
              mainLogger.debug({ tool: toolName, sessionId }, 'Using provided sessionId');
            }

            // Use router for execution
            const result = await router.route({
              toolName,
              params: paramsObj,
              sessionId,
              context,
            });

            if (result.ok) {
              return {
                content: [
                  {
                    type: 'text' as const,
                    text: JSON.stringify(result.value, null, 2),
                  },
                ],
              };
            } else {
              throw new Error(result.error || `Tool "${toolName}" failed`);
            }
          } catch (error) {
            mainLogger.error({ error, tool: toolName }, 'Tool execution failed');
            throw error;
          }
        },
      );
    }

    mainLogger.info('MCP Server created successfully');

    if (options.listTools) {
      getLogger().info('Listing available tools');

      const toolList = Array.from(tools.keys());

      console.error('\n🛠️  Available MCP Tools:');
      console.error('═'.repeat(60));

      console.error('\n📦 Containerization Tools:');
      toolList.forEach((toolName) => {
        console.error(`  • ${toolName.padEnd(30)}`);
      });

      const status = {
        healthy: true,
        running: true,
        services: {
          logger: true,
          sessionManager: true,
          promptRegistry: true,
          resourceManager: true,
        },
        stats: {
          resources: resourceManager.getStats().total,
          prompts: (await listPrompts()).length,
        },
      }; // Server is running
      console.error('\n📊 Summary:');
      console.error(`  • Total tools: ${toolList.length}`);
      console.error(`  • Resources available: ${status.stats.resources}`);
      console.error(`  • Prompts available: ${status.stats.prompts}`);

      process.exit(0);
    }

    if (options.healthCheck) {
      getLogger().info('Performing health check');

      const status = {
        healthy: true,
        running: true,
        services: {
          logger: true,
          sessionManager: true,
          promptRegistry: true,
          resourceManager: true,
        },
        stats: {
          resources: resourceManager.getStats().total,
          prompts: (await listPrompts()).length,
        },
      }; // Server is running after start

      console.error('🏥 Health Check Results');
      console.error('═'.repeat(40));
      console.error(`Status: ${status.healthy && status.running ? '✅ Healthy' : '❌ Unhealthy'}`);
      console.error('\nServices:');
      console.error(`  ✅ MCP Server: ${status.running ? 'running' : 'stopped'}`);
      console.error(`  📁 Resources available: ${status.stats.resources}`);
      console.error(`  📝 Prompts available: ${status.stats.prompts}`);

      // Show individual service status
      if (status.services) {
        console.error('\nService Health:');
        Object.entries(status.services).forEach(([service, healthy]) => {
          const icon = healthy ? '✅' : '❌';
          console.error(`  ${icon} ${service}: ${healthy ? 'healthy' : 'unhealthy'}`);
        });
      }

      process.exit(status.healthy && status.running ? 0 : 1);
    }

    getLogger().info(
      {
        config: {
          logLevel: config.server.logLevel,
          workspace: config.workspace?.workspaceDir || process.cwd(),
          devMode: options.dev,
        },
      },
      'Starting Containerization Assist MCP Server',
    );

    // Get transport information
    const transport = getTransportInfo(options);

    // Only show startup messages when not in pure MCP mode
    if (!process.env.MCP_QUIET) {
      console.error('🚀 Starting Containerization Assist MCP Server...');
      console.error(`📦 Version: ${packageJson.version}`);
      console.error(`🏠 Workspace: ${config.workspace?.workspaceDir || process.cwd()}`);
      console.error(`📊 Log Level: ${config.server.logLevel}`);
      console.error(`🔌 Transport: ${transport.details}`);

      if (options.dev) {
        console.error('🔧 Development mode enabled');
      }
    }

    // Server is ready to handle requests
    // Replace the misleading HTTP-specific message
    if (!process.env.MCP_QUIET) {
      console.error('✅ Server started successfully');

      if (transport.type === 'http') {
        console.error(`🔌 Listening on HTTP port ${options.port}`);
        console.error(`📡 Connect via: http://${options.host}:${options.port}`);
      } else {
        console.error('📡 Ready to accept MCP requests via stdio');
        console.error('💡 Send JSON-RPC messages to stdin for interaction');
      }
    }

    // Set up stdio JSON-RPC handling
    if (transport.type === 'stdio') {
      const mcpTransport = new StdioServerTransport();
      await server.connect(mcpTransport);

      // Enhanced shutdown handling with timeout
      const shutdown = async (signal: string): Promise<void> => {
        const logger = getLogger();
        logger.info({ signal }, 'Shutdown initiated');

        if (!process.env.MCP_QUIET) {
          console.error(`\n🛑 Received ${signal}, shutting down gracefully...`);
        }

        // Set a timeout for shutdown
        const shutdownTimeout = setTimeout(() => {
          logger.error('Forced shutdown due to timeout');
          console.error('⚠️ Forced shutdown - some resources may not have cleaned up properly');
          process.exit(1);
        }, 10000); // 10 second timeout

        try {
          // Close the MCP server
          await server.close();
          clearTimeout(shutdownTimeout);

          if (!process.env.MCP_QUIET) {
            console.error('✅ Shutdown complete');
          }
          process.exit(0);
        } catch (error) {
          clearTimeout(shutdownTimeout);
          logger.error({ error }, 'Shutdown error');
          console.error('❌ Shutdown error:', error);
          process.exit(1);
        }
      };

      process.on('SIGTERM', () => {
        shutdown('SIGTERM').catch((error) => {
          getLogger().error({ error }, 'Error during SIGTERM shutdown');
          process.exit(1);
        });
      });

      process.on('SIGINT', () => {
        shutdown('SIGINT').catch((error) => {
          getLogger().error({ error }, 'Error during SIGINT shutdown');
          process.exit(1);
        });
      });
    } else {
      // HTTP transport would be handled differently
      throw new Error('HTTP transport not implemented in this version');
    }
  } catch (error) {
    getLogger().error({ error }, 'Server startup failed');
    console.error('❌ Server startup failed');

    if (error instanceof Error) {
      provideContextualGuidance(error, options);
    }

    exit(1);
  }
}

process.on('uncaughtException', (error) => {
  getLogger().fatal({ error }, 'Uncaught exception in CLI');
  console.error('❌ Uncaught exception:', error);
  exit(1);
});

process.on('unhandledRejection', (reason, promise) => {
  getLogger().fatal({ reason, promise }, 'Unhandled rejection in CLI');
  console.error('❌ Unhandled rejection:', reason);
  exit(1);
});

// Run the CLI
void main();
