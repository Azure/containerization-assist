/**
 * Validate Dockerfile Tool
 *
 * Validates Dockerfile base images against configurable allowlist/denylist regex patterns.
 * Configuration is managed at the server level via environment variables.
 */

import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import { validateImageSchema, type ValidateImageResult } from './schema';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';
import { DockerfileParser } from 'dockerfile-ast';
import type { z } from 'zod';
import { appConfig } from '@/config/app-config';

const name = 'validate-dockerfile';
const description = 'Validate Dockerfile base images against allowlist/denylist patterns';
const version = '1.0.0';

interface BaseImageInfo {
  image: string;
  line: number;
}

function extractBaseImages(content: string): BaseImageInfo[] {
  const baseImages: BaseImageInfo[] = [];

  try {
    const dockerfile = DockerfileParser.parse(content);
    const instructions = dockerfile.getInstructions();

    for (const instr of instructions) {
      if (instr.getInstruction() === 'FROM') {
        const args = instr.getArguments();
        if (args && args.length > 0) {
          const imageArg = args[0];
          if (imageArg) {
            const image = imageArg.getValue();
            const line = (instr.getRange()?.start.line ?? 0) + 1;
            baseImages.push({ image, line });
          }
        }
      }
    }
  } catch (parseError) {
    throw new Error(
      `Failed to parse Dockerfile: ${parseError instanceof Error ? parseError.message : String(parseError)}`,
    );
  }

  return baseImages;
}

function validateImageAgainstRules(
  image: string,
  allowlist: string[],
  denylist: string[],
  strictMode: boolean,
): {
  allowed: boolean;
  denied: boolean;
  matchedAllowRule?: string | undefined;
  matchedDenyRule?: string | undefined;
} {
  let allowed = !strictMode;
  let denied = false;
  let matchedAllowRule: string | undefined;
  let matchedDenyRule: string | undefined;

  for (const pattern of denylist) {
    try {
      const regex = new RegExp(pattern);
      if (regex.test(image)) {
        denied = true;
        matchedDenyRule = pattern;
        break;
      }
    } catch {
      throw new Error(`Invalid denylist regex pattern: ${pattern}`);
    }
  }

  if (allowlist.length > 0) {
    for (const pattern of allowlist) {
      try {
        const regex = new RegExp(pattern);
        if (regex.test(image)) {
          allowed = true;
          matchedAllowRule = pattern;
          break;
        }
      } catch {
        throw new Error(`Invalid allowlist regex pattern: ${pattern}`);
      }
    }
  }

  return { allowed, denied, matchedAllowRule, matchedDenyRule };
}

async function run(
  input: z.infer<typeof validateImageSchema>,
  ctx: ToolContext,
): Promise<Result<ValidateImageResult>> {
  const { path, dockerfile: inputDockerfile, strictMode = false } = input;

  let content = inputDockerfile || '';

  if (path) {
    const dockerfilePath = nodePath.isAbsolute(path) ? path : nodePath.resolve(process.cwd(), path);
    try {
      content = await fs.readFile(dockerfilePath, 'utf-8');
    } catch (error) {
      return Failure(`Failed to read Dockerfile at ${dockerfilePath}: ${error}`);
    }
  }

  if (!content) {
    return Failure('Either path or dockerfile content is required');
  }

  const allowlist = appConfig.validation.imageAllowlist;
  const denylist = appConfig.validation.imageDenylist;

  ctx.logger.info(
    {
      allowlistCount: allowlist.length,
      denylistCount: denylist.length,
      strictMode,
    },
    'Validating Dockerfile base images',
  );

  let baseImages: BaseImageInfo[];
  try {
    baseImages = extractBaseImages(content);
  } catch (error) {
    return Failure(
      `Failed to extract base images: ${error instanceof Error ? error.message : String(error)}`,
    );
  }

  if (baseImages.length === 0) {
    return Failure('No FROM instructions found in Dockerfile');
  }

  const validatedImages = baseImages.map((baseImage) => {
    const validation = validateImageAgainstRules(baseImage.image, allowlist, denylist, strictMode);

    return {
      image: baseImage.image,
      line: baseImage.line,
      allowed: validation.allowed && !validation.denied,
      denied: validation.denied,
      matchedAllowRule: validation.matchedAllowRule,
      matchedDenyRule: validation.matchedDenyRule,
    };
  });

  const allowedImages = validatedImages.filter((img) => img.allowed && !img.denied);
  const deniedImages = validatedImages.filter((img) => img.denied);
  const unknownImages = validatedImages.filter((img) => !img.allowed && !img.denied);

  const violations: string[] = [];

  for (const img of deniedImages) {
    violations.push(
      `Line ${img.line}: Image '${img.image}' matches denylist pattern '${img.matchedDenyRule}'`,
    );
  }

  if (strictMode && allowlist.length > 0) {
    for (const img of unknownImages) {
      violations.push(
        `Line ${img.line}: Image '${img.image}' does not match any allowlist pattern (strict mode)`,
      );
    }
  }

  const passed = violations.length === 0;

  const workflowHints = passed
    ? {
        nextStep: 'build-image',
        message: 'All base images validated successfully. Ready to build.',
      }
    : {
        nextStep: 'fix-dockerfile',
        message: `Image validation failed with ${violations.length} violation(s). Update base images to match policy.`,
      };

  const result: ValidateImageResult = {
    success: true,
    passed,
    baseImages: validatedImages,
    summary: {
      totalImages: baseImages.length,
      allowedImages: allowedImages.length,
      deniedImages: deniedImages.length,
      unknownImages: unknownImages.length,
    },
    violations,
    workflowHints,
  };

  ctx.logger.info(
    {
      totalImages: baseImages.length,
      passed,
      violations: violations.length,
    },
    'Image validation completed',
  );

  return Success(result);
}

const tool: MCPTool<typeof validateImageSchema, ValidateImageResult> = {
  name,
  description,
  category: 'docker',
  version,
  schema: validateImageSchema,
  metadata: {
    knowledgeEnhanced: false,
    samplingStrategy: 'none',
    enhancementCapabilities: [],
  },
  run,
};

export default tool;
