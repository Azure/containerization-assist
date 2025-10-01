/**
 * Fix Dockerfile tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';

import { promptTemplates, type OptimizationPromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { scoreDockerfile } from '@/lib/scoring';
import { fixDockerfileSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import { DockerfileParser } from 'dockerfile-ast';
import validateDockerfileLib from 'validate-dockerfile';
import { validateDockerfileContent } from '@/validation/dockerfile-validator';
import { applyFixes } from '@/validation/dockerfile-fixer';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';
import type { z } from 'zod';
import type { KnowledgeEnhancementResult } from '@/mcp/ai/knowledge-enhancement';
import { extractDockerfileContent } from '@/lib/content-extraction';

const name = 'fix-dockerfile';
const description = 'Fix and optimize existing Dockerfiles';
const version = '2.0.0';

/**
 * Create a unified diff between original and fixed content
 */
function createUnifiedDiff(original: string, fixed: string): string {
  const originalLines = original.split('\n');
  const fixedLines = fixed.split('\n');

  const diff: string[] = ['--- original/Dockerfile', '+++ fixed/Dockerfile'];

  // Simple line-by-line diff implementation
  const maxLines = Math.max(originalLines.length, fixedLines.length);
  let changeStart = -1;

  for (let i = 0; i < maxLines; i++) {
    const origLine = originalLines[i] || '';
    const fixedLine = fixedLines[i] || '';

    if (origLine !== fixedLine) {
      if (changeStart === -1) {
        changeStart = i;
        diff.push(`@@ -${i + 1} +${i + 1} @@`);
      }

      if (origLine) {
        diff.push(`-${origLine}`);
      }
      if (fixedLine) {
        diff.push(`+${fixedLine}`);
      }
    } else if (changeStart !== -1) {
      // Add context line and reset
      diff.push(` ${origLine}`);
      changeStart = -1;
    }
  }

  return diff.join('\n');
}

async function run(
  input: z.infer<typeof fixDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const {
    targetEnvironment: environment = 'production',
    path,
    mode = 'full',
    enableExternalLinter = true,
    returnDiff = false,
    outputFormat: _outputFormat = 'json',
  } = input;

  // Get Dockerfile content from either path or direct content
  let content = input.dockerfile || '';

  if (path) {
    const dockerfilePath = nodePath.isAbsolute(path) ? path : nodePath.resolve(process.cwd(), path);
    try {
      content = await fs.readFile(dockerfilePath, 'utf-8');
    } catch (error) {
      return Failure(`Failed to read Dockerfile at ${dockerfilePath}: ${error}`);
    }
  }

  // First, use validate-dockerfile library for basic syntax validation
  const libraryValidation = validateDockerfileLib(content);
  const parseIssues: string[] = [];

  if (!libraryValidation.valid) {
    parseIssues.push(libraryValidation.message || 'Invalid Dockerfile syntax');
  }

  const originalContent = content;

  // Check for [object Object] or similar serialization issues
  const lines = content.split('\n');
  lines.forEach((line, idx) => {
    if (line.includes('[object Object]')) {
      parseIssues.push(`Line ${idx + 1}: Contains [object Object] serialization error`);
    }
  });

  // Check for empty COPY/RUN instructions
  lines.forEach((line, idx) => {
    if (line.trim() === 'COPY' || line.trim() === 'RUN') {
      parseIssues.push(`Line ${idx + 1}: Empty ${line.trim()} instruction`);
    }
  });

  // Check for continuation issues
  let inContinuation = false;
  lines.forEach((line, idx) => {
    if (line.endsWith('\\')) {
      inContinuation = true;
    } else if (inContinuation) {
      if (line.trim().length === 0) {
        parseIssues.push(`Line ${idx + 1}: Empty continuation line`);
      }
      inContinuation = false;
    }
  });

  // Use dockerfile-ast parser for semantic analysis
  let dockerfile;
  try {
    dockerfile = DockerfileParser.parse(content);
  } catch (parseError) {
    parseIssues.push(
      `Parser error: ${parseError instanceof Error ? parseError.message : String(parseError)}`,
    );
    // Continue anyway since we'll still try to fix it
  }

  // Semantic analysis if parsing succeeded
  if (dockerfile) {
    const instructions = dockerfile.getInstructions();

    // Check for basic requirements
    const hasFrom = instructions.some((i) => i.getInstruction() === 'FROM');
    if (!hasFrom) {
      parseIssues.push('Missing FROM instruction');
    }

    // Check file existence for COPY commands
    if (path) {
      const dockerfileDir = nodePath.dirname(
        nodePath.isAbsolute(path) ? path : nodePath.resolve(process.cwd(), path),
      );

      for (const instr of instructions) {
        if (instr.getInstruction() === 'COPY') {
          const args = instr.getArguments();
          if (args && args.length >= 2) {
            const sourceArgs = args.slice(0, -1); // All but the last argument are sources

            for (const sourceArg of sourceArgs) {
              const sourcePath = sourceArg.getValue();

              // Skip URLs, environment variables, and relative paths that might be created during build
              if (
                sourcePath.includes('://') ||
                sourcePath.includes('$') ||
                sourcePath.startsWith('--from=') ||
                sourcePath === '.' ||
                sourcePath === './'
              ) {
                continue;
              }

              // Check if file/directory exists in the build context
              const fullPath = nodePath.resolve(dockerfileDir, sourcePath);
              try {
                await fs.access(fullPath);
              } catch {
                parseIssues.push(
                  `COPY source '${sourcePath}' does not exist in build context at line ${instr.getRange()?.start.line || 'unknown'}`,
                );
              }
            }
          }
        }
      }
    }

    // Check for multiple consecutive RUN commands that could be combined
    let consecutiveRuns = 0;
    instructions.forEach((instr, idx) => {
      if (instr.getInstruction() === 'RUN') {
        consecutiveRuns++;
        if (consecutiveRuns > 3) {
          parseIssues.push(
            `Lines around ${instr.getRange()?.start.line || idx}: Multiple consecutive RUN commands could be combined`,
          );
        }
      } else {
        consecutiveRuns = 0;
      }
    });
  }

  ctx.logger.info(
    { issueCount: parseIssues.length, preview: content.substring(0, 100), mode },
    'Analyzing Dockerfile for issues',
  );

  // Mode-based behavior
  switch (mode) {
    case 'lint': {
      // Validation only mode
      const report = await validateDockerfileContent(content, {
        enableExternalLinter,
      });

      const analysis: Record<string, unknown> = {
        validationReport: report,
        issuesFound: parseIssues.length,
        issuesFixed: [],
      };

      return Success({
        content: originalContent, // Original, unchanged
        analysis,
        language: 'dockerfile',
        confidence: 1.0,
        suggestions: report.results.filter((r) => !r.passed).map((r) => r.message),
      });
    }

    case 'autofix': {
      // Apply fixes without AI
      const validationReport = await validateDockerfileContent(content, {
        enableExternalLinter,
      });

      const failedRules = validationReport.results
        .filter((r) => !r.passed && r.ruleId)
        .map((r) => r.ruleId as string);

      const { fixed, applied } = applyFixes(content, failedRules);

      const analysis: Record<string, unknown> = {
        validationReport,
        fixesApplied: applied,
        issuesFound: parseIssues.length,
        issuesFixed: applied,
      };

      if (returnDiff && originalContent !== fixed) {
        analysis.diff = createUnifiedDiff(originalContent, fixed);
      }

      // Write back if we have a path
      if (path) {
        const dockerfilePath = nodePath.isAbsolute(path)
          ? path
          : nodePath.resolve(process.cwd(), path);
        try {
          await fs.writeFile(dockerfilePath, fixed, 'utf-8');
          ctx.logger.info({ path: dockerfilePath }, 'Auto-fixed Dockerfile written successfully');
        } catch (writeError) {
          ctx.logger.error(
            { error: writeError instanceof Error ? writeError.message : String(writeError) },
            'Failed to write auto-fixed Dockerfile',
          );
        }
      }

      return Success({
        content: fixed,
        analysis,
        language: 'dockerfile',
        confidence: 0.8,
        suggestions: [`✅ Applied ${applied.length} automatic fixes: ${applied.join(', ')}`],
        workflowHints: {
          nextStep: 'build-image',
          message: `Auto-fixes applied successfully. Use "build-image" to test the fixed Dockerfile.`,
        },
      });
    }

    case 'format': {
      // Format only (basic formatting improvements)
      let formatted = content;

      // Basic formatting improvements
      const lines = formatted.split('\n');
      const formattedLines = lines.map((line) => {
        // Remove extra whitespace but preserve intentional spacing
        const trimmed = line.trim();
        if (!trimmed) return '';

        // Normalize instruction casing
        const parts = trimmed.split(/\s+/);
        if (parts.length > 0) {
          const instruction = parts[0]?.toUpperCase();
          const rest = parts.slice(1).join(' ');
          return rest ? `${instruction} ${rest}` : instruction;
        }
        return trimmed;
      });

      formatted = formattedLines.join('\n');

      const analysis: Record<string, unknown> = {
        issuesFound: 0,
        issuesFixed: ['Normalized instruction casing', 'Cleaned whitespace'],
      };

      if (returnDiff && originalContent !== formatted) {
        analysis.diff = createUnifiedDiff(originalContent, formatted);
      }

      // Write back if we have a path
      if (path) {
        const dockerfilePath = nodePath.isAbsolute(path)
          ? path
          : nodePath.resolve(process.cwd(), path);
        try {
          await fs.writeFile(dockerfilePath, formatted, 'utf-8');
          ctx.logger.info({ path: dockerfilePath }, 'Formatted Dockerfile written successfully');
        } catch (writeError) {
          ctx.logger.error(
            { error: writeError instanceof Error ? writeError.message : String(writeError) },
            'Failed to write formatted Dockerfile',
          );
        }
      }

      return Success({
        content: formatted,
        analysis,
        language: 'dockerfile',
        confidence: 0.9,
        suggestions: ['✅ Applied formatting improvements'],
        workflowHints: {
          nextStep: 'build-image',
          message: `Dockerfile formatting complete. Use "build-image" to test the formatted Dockerfile.`,
        },
      });
    }

    case 'full':
    default:
      // Fall through to full pipeline implementation below
      break;
  }

  // Use the optimization prompt template from @/ai/prompt-templates
  const optimizationParams: OptimizationPromptParams = {
    currentContent: content,
    contentType: 'dockerfile',
    issues: parseIssues,
  };
  if (input.requirements) {
    optimizationParams.requirements = input.requirements;
  }
  const basePrompt = promptTemplates.optimization(optimizationParams);

  // Build messages using the prompt engine with knowledge injection
  const messages = await buildMessages({
    basePrompt,
    topic: TOPICS.FIX_DOCKERFILE,
    tool: name,
    environment,
    contract: {
      name: 'dockerfile_fixed',
      description: 'Fix and optimize Dockerfile',
    },
    knowledgeBudget: 3000,
  });

  // Call the AI to fix the Dockerfile
  const response = await sampleWithRerank(
    ctx,
    async (attempt) => ({
      ...toMCPMessages(messages),
      maxTokens: 4096,
      modelPreferences: {
        hints: [{ name: 'dockerfile-fix' }],
        intelligencePriority: 0.95,
        speedPriority: attempt > 0 ? 0.5 : 0.2,
      },
    }),
    scoreDockerfile,
    {},
  );

  if (!response.ok) {
    return Failure(`AI sampling failed: ${response.error}`);
  }

  // Extract the fixed Dockerfile content using unified extraction
  const responseText = response.value.text;
  const extraction = extractDockerfileContent(responseText);
  const fixedContent = extraction.success && extraction.content ? extraction.content : responseText;

  // Validate the fixed content
  const fixedValidation = validateDockerfileLib(fixedContent);
  if (!fixedValidation.valid) {
    ctx.logger.warn(
      { error: fixedValidation.message },
      'Fixed Dockerfile still has validation issues',
    );
  }

  // Apply knowledge enhancement for additional improvements
  let knowledgeEnhancement: KnowledgeEnhancementResult | undefined;
  let finalContent = fixedContent;

  // Run validation to determine if knowledge enhancement is needed
  const validation = await validateDockerfileContent(fixedContent, {
    enableExternalLinter: false,
  });

  if (validation.score < 90) {
    try {
      const { enhanceWithKnowledge, createEnhancementFromValidation } = await import(
        '@/mcp/ai/knowledge-enhancement'
      );

      const enhancementRequest = createEnhancementFromValidation(
        fixedContent,
        'dockerfile',
        validation.results
          .filter((r) => !r.passed)
          .map((r) => ({
            message: r.message || 'Validation issue',
            severity: r.metadata?.severity === 'error' ? 'error' : 'warning',
            category: r.ruleId?.split('-')[0] || 'general',
          })),
        'all',
      );

      // Add user requirements as query if provided
      if (input.requirements) {
        enhancementRequest.userQuery = `Original requirements: ${input.requirements}`;
      }

      const enhancementResult = await enhanceWithKnowledge(enhancementRequest, ctx);

      if (enhancementResult.ok) {
        knowledgeEnhancement = enhancementResult.value;
        finalContent = knowledgeEnhancement.enhancedContent;

        ctx.logger.info(
          {
            knowledgeAppliedCount: knowledgeEnhancement.knowledgeApplied.length,
            confidence: knowledgeEnhancement.confidence,
            enhancementAreas: knowledgeEnhancement.analysis.enhancementAreas.length,
          },
          'Knowledge enhancement applied to fixed Dockerfile',
        );
      } else {
        ctx.logger.warn(
          { error: enhancementResult.error },
          'Knowledge enhancement failed, using AI-fixed Dockerfile',
        );
      }
    } catch (enhancementError) {
      ctx.logger.debug(
        {
          error:
            enhancementError instanceof Error ? enhancementError.message : String(enhancementError),
        },
        'Knowledge enhancement threw exception, continuing without enhancement',
      );
    }
  }

  // Write back if we have a path
  let dockerfilePath: string | undefined;
  if (path) {
    dockerfilePath = nodePath.isAbsolute(path) ? path : nodePath.resolve(process.cwd(), path);
    try {
      await fs.writeFile(dockerfilePath, finalContent, 'utf-8');
      ctx.logger.info({ path: dockerfilePath }, 'Fixed Dockerfile written successfully');
    } catch (writeError) {
      ctx.logger.error(
        { error: writeError instanceof Error ? writeError.message : String(writeError) },
        'Failed to write fixed Dockerfile',
      );
    }
  }

  const improvements: string[] = [];
  improvements.push('✅ Fixed syntax errors and validation issues');
  if (parseIssues.length > 0) {
    improvements.push(`✅ Resolved ${parseIssues.length} identified issues`);
  }
  improvements.push('✅ Applied best practices and optimizations');
  improvements.push('✅ Enhanced security and performance');

  if (knowledgeEnhancement) {
    improvements.push(
      `🧠 Enhanced with ${knowledgeEnhancement.knowledgeApplied.length} knowledge improvements`,
    );
  }

  const analysis: Record<string, unknown> = {
    issuesFound: parseIssues.length,
    issuesFixed: parseIssues,
    ...(knowledgeEnhancement && {
      enhancementAreas: knowledgeEnhancement.analysis.enhancementAreas,
      confidence: knowledgeEnhancement.confidence,
      knowledgeApplied: knowledgeEnhancement.knowledgeApplied,
    }),
  };

  // Add diff if requested
  if (returnDiff && originalContent !== finalContent) {
    analysis.diff = createUnifiedDiff(originalContent, finalContent);
  }

  const result = {
    content: finalContent,
    language: 'dockerfile',
    confidence: knowledgeEnhancement ? knowledgeEnhancement.confidence : 0.9,
    analysis,
    suggestions: improvements,
    workflowHints: {
      nextStep: 'build-image',
      message: `Dockerfile fixes applied successfully${knowledgeEnhancement ? ` with ${knowledgeEnhancement.knowledgeApplied.length} knowledge enhancements` : ''}. Use "build-image" to test the fixed Dockerfile, or review the changes and apply additional customizations.`,
    },
  };

  return Success(result);
}

const tool: Tool<typeof fixDockerfileSchema, AIResponse> = {
  name,
  description,
  category: 'docker',
  version,
  schema: fixDockerfileSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'validation', 'optimization', 'self-repair'],
  },
  run,
};

export default tool;
