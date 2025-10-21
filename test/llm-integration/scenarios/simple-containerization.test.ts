import { describe, test, beforeAll, afterAll, expect } from '@jest/globals';
import { tmpdir } from 'os';
import { join } from 'path';
import { promises as fs } from 'fs';
import { MCPTestHarness } from '../infrastructure/mcp-test-harness.js';
import { ChatClient } from '../infrastructure/chat-client.js';

describe('Simple Containerization Flow', () => {
  let testWorkspace: string;
  let harness: MCPTestHarness;
  let client: ChatClient;

  beforeAll(async () => {

    // Create test workspace
    testWorkspace = await fs.mkdtemp(join(tmpdir(), 'simple-containerization-test-'));

    // Create a simple Spring Boot project structure
    const projectPath = join(testWorkspace, 'my-app');
    await fs.mkdir(projectPath, { recursive: true });
    await fs.mkdir(join(projectPath, 'src', 'main', 'java', 'com', 'example'), { recursive: true });

    // Create pom.xml
    await fs.writeFile(join(projectPath, 'pom.xml'), `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.org/xsd/maven-4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>my-app</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>

    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.1.5</version>
    </parent>

    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
    </dependencies>
</project>`);

    // Create main application class
    await fs.writeFile(join(projectPath, 'src/main/java/com/example/Application.java'), `package com.example;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
public class Application {
    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }
}

@RestController
class HelloController {
    @GetMapping("/")
    public String hello() {
        return "Hello World!";
    }
}`);

    // Initialize MCP harness with only essential tools
    harness = new MCPTestHarness();

    await harness.createTestServer('simple-test', {
      workingDirectory: testWorkspace,
      enableTools: ['analyze-repo', 'generate-dockerfile']
    });

    // Initialize chat client
    client = new ChatClient({});
    const isValidated = await client.validateConnection();

    if (!isValidated) {
      throw new Error('LLM client validation failed');
    }

    console.log('LLM client validated successfully');
  }, 30000);

  afterAll(async () => {
    if (harness) {
      await harness.stopServer('simple-test');
    }

    if (testWorkspace) {
      await fs.rm(testWorkspace, { recursive: true, force: true });
    }
  });

  test('should containerize application end-to-end like GitHub Copilot Chat', async () => {
    const projectPath = join(testWorkspace, 'my-app');

    console.log('🤖 Testing autonomous agent behavior like GitHub Copilot Chat');
    console.log(`📁 Project path: ${projectPath}`);

    // Get available tools from the harness
    const availableTools = harness.getAvailableTools('simple-test');

    // Create MCP tool executor that uses our test harness
    const mcpToolExecutor = async (toolName: string, params: any) => {
      console.log(`🔧 Executing MCP tool: ${toolName} with params:`, params);
      const toolCall = { name: toolName, arguments: params, id: `tool-${Date.now()}` };
      const result = await harness.executeToolCall('simple-test', toolCall);
      console.log(`✅ Tool ${toolName} result:`, result.content ? 'Success' : 'Error');
      return result.content || result.error;
    };

    // This is what happens when user asks: "Containerize my application"
    // Using autonomous agent that should behave like GitHub Copilot Chat
    const result = await client.executeAutonomously(
      `I need help containerizing my Spring Boot application. The project is located at ${projectPath}.

Please analyze the repository first to understand the structure, then generate an appropriate Dockerfile for production deployment.

After you have the information you need, please create the actual Dockerfile in the project directory.`,
      projectPath, // Working directory where files should be created
      mcpToolExecutor,
      availableTools,
      {
        maxSteps: 5,
        temperature: 0.1
      }
    );

    console.log('🎯 Autonomous execution completed');
    console.log('📝 LLM Response:', result.response);
    console.log('🔧 Tools executed:', result.toolCallsExecuted.map(tc => tc.toolName));
    console.log('📁 Files created:', result.filesCreated);

    // **KEY TEST**: Verify LLM actually created the Dockerfile file autonomously
    // This is what GitHub Copilot Chat would do - create the actual file
    const dockerfilePath = join(projectPath, 'Dockerfile');

    // First check if the agent reported creating a Dockerfile
    expect(result.filesCreated.length).toBeGreaterThan(0);
    expect(result.filesCreated.some(file => file.includes('Dockerfile'))).toBe(true);

    // Then verify the file actually exists and has valid content
    try {
      const dockerfileContent = await fs.readFile(dockerfilePath, 'utf8');
      console.log('✅ Dockerfile created successfully by autonomous agent!');
      console.log('📄 Complete Dockerfile content:');
      console.log('='.repeat(60));
      console.log(dockerfileContent);
      console.log('='.repeat(60));

      // Basic validation of Dockerfile content
      expect(dockerfileContent).toContain('FROM');
      expect(dockerfileContent).toMatch(/openjdk|eclipse-temurin|java/i);
      expect(dockerfileContent).toContain('COPY');

      // Verify the agent executed the expected tools
      expect(result.toolCallsExecuted.some(tc => tc.toolName === 'analyze-repo')).toBe(true);
      expect(result.toolCallsExecuted.some(tc => tc.toolName === 'generate-dockerfile')).toBe(true);

    } catch (error) {
      console.log('❌ Dockerfile was not created by autonomous agent');
      console.log('This means the agent is not behaving like GitHub Copilot Chat would');
      console.log('Agent reported creating:', result.filesCreated);
      throw new Error('Autonomous agent failed to create Dockerfile after executing tools. Expected behavior: Agent should create files based on tool outputs, just like GitHub Copilot Chat.');
    }

  }, 120000); // 2 minute timeout
});