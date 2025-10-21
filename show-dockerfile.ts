import { join } from 'path';
import { promises as fs } from 'fs';
import { tmpdir } from 'os';
import { MCPTestHarness } from './test/llm-integration/infrastructure/mcp-test-harness.ts';
import { ChatClient } from './test/llm-integration/infrastructure/chat-client.ts';

async function showDockerfileCreation() {
  console.log('🚀 Starting autonomous agent to create Dockerfile...\n');

  // Create test project directory
  const tempDir = await fs.mkdtemp(join(tmpdir(), 'dockerfile-demo-'));
  const projectPath = join(tempDir, 'demo-app');

  await fs.mkdir(projectPath, { recursive: true });

  // Create minimal Spring Boot project structure
  const pomXml = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>demo-app</artifactId>
  <version>1.0.0</version>
  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
    <version>3.2.0</version>
  </parent>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
  </dependencies>
</project>`;

  await fs.writeFile(join(projectPath, 'pom.xml'), pomXml);

  // Setup MCP test harness
  const harness = new MCPTestHarness();
  await harness.createTestServer('demo-test', {
    workingDirectory: tempDir,
    enableTools: ['analyze-repo', 'generate-dockerfile']
  });

  // Create chat client
  const client = new ChatClient('gpt-4o');

  // Validate connection - fail fast if no connection
  console.log('🔗 Validating connection to local Copilot API proxy...');
  const isConnected = await client.validateConnection();
  if (!isConnected) {
    console.log('❌ Could not connect to local Copilot API proxy at http://localhost:4141');
    console.log('💡 Make sure the copilot-api proxy is running on port 4141');
    console.log('💡 You can also set OPENAI_BASE_URL and OPENAI_API_KEY environment variables');
    process.exit(1);
  }
  console.log('✅ Connected to local Copilot API proxy');

  // Create tool executor
  const mcpToolExecutor = async (toolName, params) => {
    const toolCall = { name: toolName, arguments: params, id: `tool-${Date.now()}` };
    const result = await harness.executeToolCall('demo-test', toolCall);
    return result.content || result.error;
  };

  const availableTools = harness.getAvailableTools('demo-test');

  console.log('🤖 Asking autonomous agent: "Please containerize my Spring Boot application"');
  console.log('📁 Project location:', projectPath);
  console.log('');

  // Execute autonomous agent
  const result = await client.executeAutonomously(
    `I need help containerizing my Spring Boot application. The project is located at ${projectPath}.

Please analyze the repository first to understand the structure, then generate an appropriate Dockerfile for production deployment.

After you have the information you need, please create the actual Dockerfile in the project directory.`,
    projectPath,
    mcpToolExecutor,
    availableTools,
    {
      maxSteps: 5,
      temperature: 0.1
    }
  );

  console.log('🎯 Autonomous execution completed!');
  console.log('🔧 Tools executed:', result.toolCallsExecuted.map(tc => tc.toolName));
  console.log('📁 Files created:', result.filesCreated);
  console.log('');

  // Read and display the created Dockerfile
  const dockerfilePath = join(projectPath, 'Dockerfile');
  try {
    const dockerfileContent = await fs.readFile(dockerfilePath, 'utf8');
    console.log('📄 Here is the Dockerfile the autonomous agent created:');
    console.log('═'.repeat(60));
    console.log(dockerfileContent);
    console.log('═'.repeat(60));
  } catch (error) {
    console.log('❌ Could not read Dockerfile:', error.message);
  }

  // Cleanup
  await harness.cleanup();
  console.log('\n✅ Demo completed successfully!');
}

showDockerfileCreation().catch(console.error);