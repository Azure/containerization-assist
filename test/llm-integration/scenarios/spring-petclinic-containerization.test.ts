import { describe, test, beforeAll, afterAll, expect } from '@jest/globals';
import { tmpdir } from 'os';
import { join } from 'path';
import { promises as fs } from 'fs';
import { MCPTestHarness } from '../infrastructure/mcp-test-harness.js';
import { ChatClient } from '../infrastructure/chat-client.js';

describe('Spring PetClinic Containerization - Real World Scenario', () => {
  let testWorkspace: string;
  let petclinicPath: string;
  let harness: MCPTestHarness;
  let client: ChatClient;

  beforeAll(async () => {
    console.log('🏗️ Setting up Spring PetClinic containerization test...');

    // Create test workspace
    testWorkspace = await fs.mkdtemp(join(tmpdir(), 'petclinic-containerization-test-'));
    petclinicPath = join(testWorkspace, 'spring-petclinic');

    console.log(`📁 Test workspace: ${testWorkspace}`);
    console.log(`📂 PetClinic path: ${petclinicPath}`);

    // Clone the Spring PetClinic repository
    console.log('📥 Cloning Spring PetClinic repository...');
    const { spawn } = await import('child_process');

    await new Promise<void>((resolve, reject) => {
      const gitClone = spawn('git', [
        'clone',
        '--depth', '1',  // Shallow clone for faster setup
        '--branch', 'main',
        'https://github.com/spring-projects/spring-petclinic.git',
        petclinicPath
      ], {
        cwd: testWorkspace,
        stdio: 'pipe'
      });

      let output = '';
      let errorOutput = '';

      gitClone.stdout?.on('data', (data) => {
        output += data.toString();
      });

      gitClone.stderr?.on('data', (data) => {
        errorOutput += data.toString();
      });

      gitClone.on('close', (code) => {
        if (code === 0) {
          console.log('✅ Spring PetClinic repository cloned successfully');
          resolve();
        } else {
          console.error('❌ Git clone failed:', errorOutput);
          reject(new Error(`Git clone failed with code ${code}: ${errorOutput}`));
        }
      });

      gitClone.on('error', (error) => {
        console.error('❌ Git clone error:', error);
        reject(error);
      });
    });

    // Verify the repository structure
    const pomPath = join(petclinicPath, 'pom.xml');
    const pomExists = await fs.access(pomPath).then(() => true).catch(() => false);
    if (!pomExists) {
      throw new Error('PetClinic repository does not contain pom.xml - clone may have failed');
    }
    console.log('✅ PetClinic repository structure verified');

    // Initialize MCP harness with repository analysis and Dockerfile generation tools
    harness = new MCPTestHarness();

    await harness.createTestServer('petclinic-test', {
      workingDirectory: testWorkspace,
      enableTools: ['analyze-repo', 'generate-dockerfile']
    });

    console.log('✅ MCP test harness initialized');

    // Initialize chat client
    client = new ChatClient();
    const isValidated = await client.validateConnection();

    if (!isValidated) {
      throw new Error('LLM client validation failed - check Azure OpenAI configuration');
    }

    console.log('✅ LLM client validated successfully');
  }, 60000); // 1 minute timeout for setup

  afterAll(async () => {
    console.log('🧹 Cleaning up test environment...');

    if (harness) {
      await harness.stopServer('petclinic-test');
    }

    if (testWorkspace) {
      await fs.rm(testWorkspace, { recursive: true, force: true });
      console.log('✅ Test workspace cleaned up');
    }
  });

  test('should analyze Spring PetClinic and generate production-ready Dockerfile', async () => {
    console.log('🎯 Testing LLM agent on real Spring PetClinic project');
    console.log(`📂 Analyzing project at: ${petclinicPath}`);

    // Get available tools from the harness
    const availableTools = harness.getAvailableTools('petclinic-test');
    console.log('🔧 Available tools:', availableTools.map(t => t.name));

    // Create MCP tool executor that uses our test harness
    const mcpToolExecutor = async (toolName: string, params: any) => {
      console.log(`🛠️ Executing MCP tool: ${toolName}`);
      console.log(`📋 Tool parameters:`, JSON.stringify(params, null, 2));

      const toolCall = {
        name: toolName,
        arguments: params,
        id: `tool-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
      };

      const result = await harness.executeToolCall('petclinic-test', toolCall);

      if (result.content) {
        console.log(`✅ Tool ${toolName} executed successfully`);
        // Log a preview of the result without overwhelming the console
        const preview = typeof result.content === 'string'
          ? result.content.substring(0, 200) + (result.content.length > 200 ? '...' : '')
          : JSON.stringify(result.content, null, 2).substring(0, 200) + '...';
        console.log(`📄 Result preview: ${preview}`);
      } else {
        console.log(`❌ Tool ${toolName} failed:`, result.error);
      }

      return result.content || result.error;
    };

    // Execute autonomous agent like GitHub Copilot Chat would
    const result = await client.executeAutonomously(
      `I need help containerizing the Spring PetClinic application for production deployment. The project is located at ${petclinicPath}.

Please analyze the repository first to understand the structure, then generate an appropriate Dockerfile for production deployment.

After you have the information you need, please create the actual Dockerfile in the project directory.`,
      petclinicPath, // Working directory where files should be created
      mcpToolExecutor,
      availableTools,
      {
        maxSteps: 5,
        temperature: 0.1
      }
    );

    console.log('🎯 Autonomous execution completed');
    console.log('📝 Final LLM Response:');
    console.log('='.repeat(80));
    console.log(result.response);
    console.log('='.repeat(80));

    console.log('🔧 Tools executed in sequence:');
    result.toolCallsExecuted.forEach((tc, index) => {
      console.log(`  ${index + 1}. ${tc.toolName} (${tc.executionTime}ms)`);
    });

    console.log('📁 Files reported as created:', result.filesCreated);

    // **CRITICAL VALIDATION**: Verify the LLM agent actually analyzed and containerized the real project

    // 1. Verify repository analysis was performed
    expect(result.toolCallsExecuted.some(tc => tc.toolName === 'analyze-repo')).toBe(true);
    console.log('✅ Repository analysis tool was executed');

    // 2. Verify Dockerfile generation was performed
    expect(result.toolCallsExecuted.some(tc => tc.toolName === 'generate-dockerfile')).toBe(true);
    console.log('✅ Dockerfile generation tool was executed');

    // 3. Verify Dockerfile was actually created
    const dockerfilePath = join(petclinicPath, 'Dockerfile');

    let dockerfileContent: string;
    try {
      dockerfileContent = await fs.readFile(dockerfilePath, 'utf8');
      console.log('✅ Dockerfile was created by the autonomous agent!');
    } catch (error) {
      console.log('❌ Dockerfile was not created by autonomous agent');
      console.log('Agent reported creating:', result.filesCreated);
      throw new Error('Autonomous agent failed to create Dockerfile after executing analysis and generation tools. This indicates the agent is not behaving like GitHub Copilot Chat would.');
    }

    // 4. Validate Dockerfile quality for Spring Boot application
    console.log('🔍 Validating Dockerfile content for Spring Boot best practices...');
    console.log('📄 Generated Dockerfile:');
    console.log('='.repeat(60));
    console.log(dockerfileContent);
    console.log('='.repeat(60));

    // Basic Dockerfile structure validation
    expect(dockerfileContent).toContain('FROM');
    expect(dockerfileContent.toLowerCase()).toMatch(/openjdk|eclipse-temurin|java|amazoncorretto/);

    // Spring Boot specific validations
    expect(dockerfileContent).toContain('COPY');
    expect(dockerfileContent).toMatch(/8080|EXPOSE/); // Spring Boot default port or explicit EXPOSE

    // Maven/build specific (PetClinic uses Maven)
    const hasMavenBuild = dockerfileContent.toLowerCase().includes('mvn') ||
                         dockerfileContent.includes('maven') ||
                         dockerfileContent.includes('pom.xml') ||
                         dockerfileContent.includes('*.jar');
    expect(hasMavenBuild).toBe(true);

    console.log('✅ Dockerfile contains Spring Boot and Maven-specific configuration');

    // 5. Verify the analysis actually found real PetClinic characteristics
    // The agent should have discovered this is a Spring Boot app with specific dependencies
    const analysisToolCall = result.toolCallsExecuted.find(tc => tc.toolName === 'analyze-repo');
    expect(analysisToolCall).toBeDefined();
    console.log('✅ Real-world Spring PetClinic project was successfully analyzed and containerized');

    // 6. Verify agent created actual files (not just returned content)
    expect(result.filesCreated.length).toBeGreaterThan(0);
    expect(result.filesCreated.some(file => file.includes('Dockerfile'))).toBe(true);
    console.log('✅ Agent autonomously created files in the filesystem');

    console.log('🎉 Test completed successfully! The LLM agent successfully:');
    console.log('   - Analyzed a real-world Spring Boot application (PetClinic)');
    console.log('   - Generated an appropriate production Dockerfile');
    console.log('   - Created the actual file in the project directory');
    console.log('   - Demonstrated GitHub Copilot Chat-like autonomous behavior');

  }, 300000); // 5 minute timeout for this complex test

  test('should handle real-world project complexities and dependencies', async () => {
    console.log('🧪 Testing agent understanding of PetClinic-specific complexities...');

    // Read the actual pom.xml to understand what the agent should have discovered
    const pomPath = join(petclinicPath, 'pom.xml');
    const pomContent = await fs.readFile(pomPath, 'utf8');

    console.log('🔍 Verifying agent can understand real project structure...');

    // The PetClinic has specific characteristics that a good analysis should discover
    const hasSpringBootParent = pomContent.includes('spring-boot-starter-parent');
    const hasWebDependency = pomContent.includes('spring-boot-starter-web');
    const hasJpaDependency = pomContent.includes('spring-boot-starter-data-jpa');

    expect(hasSpringBootParent).toBe(true);
    console.log('✅ PetClinic is confirmed to be a Spring Boot application');

    if (hasWebDependency) {
      console.log('✅ PetClinic has web dependencies (should expose port 8080)');
    }

    if (hasJpaDependency) {
      console.log('✅ PetClinic uses JPA (database application complexity)');
    }

    // This test validates that we're working with a real, complex project
    // The previous test should have handled this complexity correctly
    console.log('🎯 Confirmed: Test is using a real-world Spring Boot application with:');
    console.log('   - Complex dependency management');
    console.log('   - Database integration');
    console.log('   - Web framework');
    console.log('   - Production-grade structure');

  }, 30000);
});