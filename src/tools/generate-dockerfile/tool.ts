import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
// Removed unused imports - now using multi-step-generator
import { generateDockerfileSchema, type GenerateDockerfileParams } from './schema';
import type { AIResponse } from '../ai-response-types';
import { getSession, updateSession } from '@/mcp/tool-session-helpers';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';
import { DockerfileParser } from 'dockerfile-ast';
import {
  generateBaseImage,
  generateDependencies,
  generateBuildSteps,
  generateRuntime,
  combineDockerfileSteps,
} from './multi-step-generator';

export async function generateDockerfile(
  params: GenerateDockerfileParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = generateDockerfileSchema.parse(params);
  const { multistage, securityHardening, optimization, sessionId, path } = validatedParams;

  // Retrieve repository analysis from session if sessionId is provided
  let language = 'auto-detect';
  let framework = null;
  let dependencies: string[] = [];
  let ports = [8080];
  let _promptPrefix = '';

  if (sessionId) {
    const sessionResult = await getSession(sessionId, context);
    if (sessionResult.ok && sessionResult.value.state.metadata?.repositoryAnalysis) {
      const analysis = sessionResult.value.state.metadata.repositoryAnalysis as Record<
        string,
        unknown
      >;

      // Use actual values from the analysis
      language = typeof analysis.language === 'string' ? analysis.language : 'auto-detect';
      framework = typeof analysis.framework === 'string' ? analysis.framework : null;
      dependencies = Array.isArray(analysis.dependencies)
        ? (analysis.dependencies as string[])
        : [];
      ports = Array.isArray(analysis.suggestedPorts)
        ? (analysis.suggestedPorts as number[])
        : [8080];

      // Create a detailed prompt with the actual analysis data
      _promptPrefix = `Repository Analysis Results:
`;
      _promptPrefix += `- Language: ${language} ${typeof analysis.languageVersion === 'string' ? `(${analysis.languageVersion})` : ''}\n`;
      if (framework) {
        _promptPrefix += `- Framework: ${framework} ${typeof analysis.frameworkVersion === 'string' ? `(${analysis.frameworkVersion})` : ''}\n`;
      }
      const buildSystem = analysis.buildSystem as Record<string, unknown> | undefined;
      _promptPrefix += `- Build System: ${buildSystem?.type || 'unknown'}\n`;
      if (dependencies.length > 0) {
        _promptPrefix += `- Key Dependencies: ${dependencies.slice(0, 5).join(', ')}${dependencies.length > 5 ? '...' : ''}\n`;
      }
      if (typeof analysis.entryPoint === 'string') {
        _promptPrefix += `- Entry Point: ${analysis.entryPoint}\n`;
      }
      _promptPrefix += `\nGenerate a Dockerfile based on this analysis.\n\n`;

      context.logger.info(
        { sessionId, language, framework },
        'Retrieved repository analysis from session',
      );
    } else {
      context.logger.warn({ sessionId }, 'Session not found or no analysis data available');
      _promptPrefix = `Warning: Could not retrieve analysis data from session ${sessionId}. Analyze the repository to determine the best configuration.\n\n`;
    }
  } else if (path) {
    _promptPrefix = `Analyze the repository at ${path} to detect the technology stack, dependencies, and requirements.\n\n`;
  }

  // Use multi-step approach to avoid timeout
  const environment = validatedParams.environment || 'production';

  context.logger.info(
    { language, framework, multistage, optimization },
    'Starting multi-step Dockerfile generation to avoid timeout',
  );

  let dockerfileContent = '';

  try {
    // Step 1: Generate base image and initial setup
    context.logger.info('Step 1/4: Generating base image instructions');
    const baseResult = await generateBaseImage(language, framework, context, environment);
    if (!baseResult.ok) {
      return Failure(`Failed to generate base image: ${baseResult.error}`);
    }

    // Step 2: Generate dependency installation
    context.logger.info('Step 2/4: Generating dependency installation');
    const depsResult = await generateDependencies(
      language,
      framework,
      baseResult.value.content,
      dependencies,
      context,
      environment,
    );
    if (!depsResult.ok) {
      return Failure(`Failed to generate dependencies: ${depsResult.error}`);
    }

    // Step 3: Generate build steps
    context.logger.info('Step 3/4: Generating build steps');
    const buildResult = await generateBuildSteps(language, framework, context, environment);
    if (!buildResult.ok) {
      return Failure(`Failed to generate build steps: ${buildResult.error}`);
    }

    // Step 4: Generate runtime configuration
    context.logger.info('Step 4/4: Generating runtime configuration');
    const runtimeResult = await generateRuntime(
      language,
      framework,
      ports,
      securityHardening || false,
      typeof optimization === 'string' ? optimization : undefined,
      context,
      environment,
    );
    if (!runtimeResult.ok) {
      return Failure(`Failed to generate runtime: ${runtimeResult.error}`);
    }

    // Combine all steps
    dockerfileContent = combineDockerfileSteps(
      baseResult.value.content,
      depsResult.value.content,
      buildResult.value.content,
      runtimeResult.value.content,
    );

    context.logger.info(
      { contentLength: dockerfileContent.length },
      'Successfully generated Dockerfile using multi-step approach',
    );
  } catch (stepError) {
    // Handle any errors during multi-step generation
    const errorMessage = stepError instanceof Error ? stepError.message : String(stepError);
    context.logger.error({ error: errorMessage }, 'Multi-step generation failed');
    return Failure(`Multi-step Dockerfile generation failed: ${errorMessage}`);
  }

  // Response object is no longer needed with multi-step approach

  // Return result with workflow hints
  try {
    // dockerfileContent is already set from multi-step generation
    const responseText = dockerfileContent;

    // Validate the extracted Dockerfile using proper parser
    try {
      const dockerfile = DockerfileParser.parse(dockerfileContent);

      // Check for basic requirements
      const instructions = dockerfile.getInstructions();
      const hasFrom = instructions.some((i) => i.getInstruction() === 'FROM');

      if (!hasFrom) {
        context.logger.error(
          {
            responseText: responseText.substring(0, 500),
            extractedContent: dockerfileContent.substring(0, 500),
          },
          'Dockerfile missing FROM instruction',
        );
        return Failure('Generated Dockerfile is missing FROM instruction');
      }

      // Check for syntax errors
      const syntaxErrors = dockerfile.getComments().filter((c) => c.getContent().includes('error'));
      if (syntaxErrors.length > 0) {
        context.logger.warn(
          { errors: syntaxErrors.map((e) => e.getContent()) },
          'Dockerfile has potential syntax issues',
        );
      }
    } catch (parseError) {
      context.logger.error(
        {
          error: parseError instanceof Error ? parseError.message : String(parseError),
          dockerfileContent: dockerfileContent.substring(0, 500),
        },
        'Failed to parse Dockerfile',
      );
      return Failure(
        `Generated Dockerfile has invalid syntax: ${parseError instanceof Error ? parseError.message : 'Unknown error'}`,
      );
    }

    // Log what we extracted
    context.logger.info(
      {
        originalLength: responseText.length,
        extractedLength: dockerfileContent.length,
        preview: dockerfileContent.substring(0, 100),
      },
      'Extracted Dockerfile content',
    );

    // Determine where to write the Dockerfile
    let dockerfilePath = '';
    if (sessionId) {
      // Get the analyzed path from session
      const sessionResult = await getSession(sessionId, context);
      if (sessionResult.ok && sessionResult.value.state.metadata?.analyzedPath) {
        dockerfilePath = nodePath.join(
          sessionResult.value.state.metadata.analyzedPath as string,
          'Dockerfile',
        );
      }
    } else if (validatedParams.path) {
      // Use the provided path
      let targetPath = validatedParams.path;
      if (!nodePath.isAbsolute(targetPath)) {
        targetPath = nodePath.resolve(process.cwd(), targetPath);
      }
      dockerfilePath = nodePath.join(targetPath, 'Dockerfile');
    } else {
      // Default to current directory
      dockerfilePath = nodePath.join(process.cwd(), 'Dockerfile');
    }

    // Write the Dockerfile to disk
    await fs.writeFile(dockerfilePath, dockerfileContent, 'utf-8');
    context.logger.info({ dockerfilePath }, 'Dockerfile written to disk');

    // Store the Dockerfile path in session if we have a session
    if (sessionId) {
      await updateSession(
        sessionId,
        {
          metadata: {
            dockerfilePath,
            dockerfileContent,
          },
          current_step: 'generate-dockerfile',
        },
        context,
      );
    }

    return Success({
      dockerfile: dockerfileContent,
      dockerfilePath,
      sessionId: sessionId || validatedParams.sessionId,
      workflowHints: {
        nextStep: 'build-image',
        message: `Dockerfile generated and saved to ${dockerfilePath}. Use "build-image" with the sessionId to build the Docker image.`,
      },
    });
  } catch (e) {
    const error = e as Error;
    return Failure(`Dockerfile generation failed: ${error.message}`);
  }
}

export const metadata = {
  name: 'generate-dockerfile',
  description: 'Generate optimized Dockerfiles for containerization',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
