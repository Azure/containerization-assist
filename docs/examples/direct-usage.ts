/**
 * Example: Direct tool usage without MCP server
 * Shows how to use Container Assist tools directly
 */

import {
  createApp,
  type Tool
} from '@azure/containerization-assist-mcp';

async function directUsageExample() {
  console.log('=== Direct Tool Usage Example ===\n');

  // Create Container Assist app
  const app = createApp({
    toolAliases: {
      'analyze-repo': 'direct_analyze',
      'generate-dockerfile': 'direct_dockerfile'
    }
  });

  console.log('Container Assist app created with custom tool names\n');

  // Get tool information
  const tools = app.listTools();
  console.log('Available tools:');
  tools.slice(0, 5).forEach(tool => {
    console.log(`- ${tool.name}: ${tool.description}`);
  });
  console.log(`... and ${tools.length - 5} more tools\n`);

  // Health check
  const health = app.healthCheck();
  console.log(`App Status: ${health.status} - ${health.message}\n`);

  // Execute tool directly
  try {
    console.log('Attempting direct tool execution...');
    const result = await app.execute('direct_analyze', {
      path: '/example/repo'
    });

    if (result.ok) {
      console.log('✅ Direct tool execution successful');
      console.log('Result:', JSON.stringify(result.value, null, 2));
    } else {
      console.log('⚠️ Tool execution failed:', result.error);
    }
  } catch (error) {
    console.log('⚠️ Direct execution requires proper Docker/filesystem setup');
    console.log('This is expected in a demo environment\n');
  }

  // Chain tool execution example
  try {
    console.log('Example: Chaining tool operations...');

    // First analyze repo (mock)
    console.log('1. Analyzing repository...');
    console.log('2. Generating Dockerfile based on analysis...');

    const dockerfileResult = await app.execute('direct_dockerfile', {
      path: '/example/repo',
      language: 'typescript'
    });

    if (dockerfileResult.ok) {
      console.log('✅ Dockerfile generation successful');
    }
  } catch (error) {
    console.log('   (Would work with proper repository setup)');
  }
}

// List all available tools
function listAvailableTools() {
  console.log('\n=== Available Tools ===\n');

  const app = createApp();
  const tools = app.listTools();

  console.log(`Total tools available: ${tools.length}\n`);

  // Group tools by category (basic grouping)
  const categories = {
    analysis: tools.filter(t => t.name.includes('analyze') || t.name.includes('scan')),
    generation: tools.filter(t => t.name.includes('generate') || t.name.includes('create')),
    build: tools.filter(t => t.name.includes('build') || t.name.includes('tag') || t.name.includes('push')),
    deployment: tools.filter(t => t.name.includes('deploy') || t.name.includes('verify') || t.name.includes('prepare')),
    other: tools.filter(t =>
      !t.name.includes('analyze') && !t.name.includes('scan') &&
      !t.name.includes('generate') && !t.name.includes('create') &&
      !t.name.includes('build') && !t.name.includes('tag') && !t.name.includes('push') &&
      !t.name.includes('deploy') && !t.name.includes('verify') && !t.name.includes('prepare')
    )
  };

  Object.entries(categories).forEach(([category, categoryTools]) => {
    if (categoryTools.length > 0) {
      console.log(`${category.toUpperCase()} TOOLS:`);
      categoryTools.forEach(tool => {
        console.log(`  ${tool.name}: ${tool.description}`);
      });
      console.log('');
    }
  });
}

// Demo with custom aliases
function demonstrateAliases() {
  console.log('\n=== Tool Aliasing Demo ===\n');

  // Create app with various aliasing strategies
  const strategies = [
    {
      name: 'Descriptive Names',
      aliases: {
        'analyze-repo': 'repository_analyzer',
        'build-image': 'docker_image_builder',
        'deploy': 'kubernetes_deployer'
      }
    },
    {
      name: 'Short Names',
      aliases: {
        'analyze-repo': 'analyze',
        'generate-dockerfile': 'dockerfile',
        'build-image': 'build'
      }
    },
    {
      name: 'Prefixed Names',
      aliases: {
        'analyze-repo': 'ca_analyze_repo',
        'build-image': 'ca_build_image',
        'deploy': 'ca_deploy'
      }
    }
  ];

  strategies.forEach(strategy => {
    console.log(`${strategy.name}:`);
    const app = createApp({ toolAliases: strategy.aliases });
    const aliasedTools = app.listTools().filter(tool =>
      Object.values(strategy.aliases).includes(tool.name)
    );

    aliasedTools.forEach(tool => {
      const original = Object.keys(strategy.aliases).find(k =>
        strategy.aliases[k as keyof typeof strategy.aliases] === tool.name
      );
      console.log(`  ${tool.name} (was: ${original})`);
    });
    console.log('');
  });
}

// Run examples
if (import.meta.url === `file://${process.argv[1]}`) {
  listAvailableTools();
  demonstrateAliases();
  directUsageExample().catch(console.error);
}

export { directUsageExample, listAvailableTools };