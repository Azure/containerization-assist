/**
 * Example: Direct tool usage without MCP server
 * Shows how to use Container Assist tools directly
 */

import { 
  analyzeRepo, 
  generateDockerfile,
  tools,
  createSession 
} from '@azure/containerization-assist-mcp';

async function directUsageExample() {
  console.log('=== Direct Tool Usage Example ===\n');
  
  // Create a session for tracking
  const sessionId = createSession();
  console.log(`Created session: ${sessionId}\n`);
  
  // Access tool metadata
  console.log('Tool Information:');
  console.log('- Name:', analyzeRepo.name);
  console.log('- Title:', analyzeRepo.metadata.title);
  console.log('- Description:', analyzeRepo.metadata.description);
  console.log('\n');
  
  // Execute tool directly
  try {
    console.log('Analyzing repository...');
    const result = await analyzeRepo.handler({
      repo_path: '/path/to/your/repo',
      session_id: sessionId
    });
    
    console.log('Analysis Result:');
    console.log(result.content[0]?.text);
    
    // Use the analysis to generate a Dockerfile
    console.log('\nGenerating Dockerfile...');
    const dockerfileResult = await generateDockerfile.handler({
      repo_path: '/path/to/your/repo',
      session_id: sessionId
    });
    
    console.log('Generated Dockerfile:');
    console.log(dockerfileResult.content[0]?.text);
    
  } catch (error) {
    console.error('Error:', error);
  }
}

// List all available tools
function listAvailableTools() {
  console.log('\n=== Available Tools ===\n');
  
  Object.entries(tools).forEach(([key, tool]) => {
    console.log(`${key}:`);
    console.log(`  Name: ${tool.name}`);
    console.log(`  Title: ${tool.metadata.title}`);
    console.log(`  Description: ${tool.metadata.description}`);
    console.log('');
  });
}

// Run examples
if (import.meta.url === `file://${process.argv[1]}`) {
  listAvailableTools();
  directUsageExample().catch(console.error);
}

export { directUsageExample, listAvailableTools };