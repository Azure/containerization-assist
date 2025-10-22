/**
 * Schema definition for analyze-repo tool
 */

import { z } from 'zod';
import { repositoryPath, analysisOptions } from '../shared/schemas';

const moduleInfo = z.object({
  name: z.string().describe('The name of the module'),
  modulePath: z
    .string()
    .describe(
      'Absolute path to module root. Paths are automatically normalized to forward slashes on all platforms.',
    ),
  dockerfilePath: z.string().optional().describe('Path where the Dockerfile should be generated'),
  language: z
    .enum(['java', 'dotnet', 'javascript', 'typescript', 'python', 'rust', 'go', 'other'])
    .optional()
    .describe('Primary programming language used in the module'),
  languageVersion: z.string().optional(),
  frameworks: z
    .array(
      z.object({
        name: z
          .string()
          .describe('Frameworks used in project like Java Spring, SpringBoot, Hibernate etc.'),
        version: z.string().optional(),
      }),
    )
    .optional(),
  buildSystem: z
    .object({
      type: z.string().optional(),
      configFile: z.string().optional(),
    })
    .optional()
    .describe('Build system information like Maven or Gradle'),
  dependencies: z
    .array(z.string())
    .optional()
    .describe('List of module dependencies including database drivers and system libraries'),
  ports: z.array(z.number()).optional(),
  entryPoint: z.string().optional(),
});
export type ModuleInfo = z.infer<typeof moduleInfo>;

export const analyzeRepoSchema = z.object({
  repositoryPath,
  ...analysisOptions,
  modules: z
    .array(moduleInfo)
    .optional()
    .describe('Optional pre-analyzed modules. If not provided, AI will analyze the repository.'),
});

export interface RepositoryAnalysis {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the analysis outcome, detected modules, and next steps.
   * @example "✅ Analyzed repository at /app/myproject. Detected node 18.x project with Express framework. Ready for Dockerfile generation."
   */
  summary?: string;
  modules?: ModuleInfo[];
  isMonorepo?: boolean;
  analyzedPath?: string;
  // Fields from AI response (for parsing)
  language?: string;
  languageVersion?: string;
  framework?: string;
  frameworkVersion?: string;
  buildSystem?: {
    type?: string;
    buildFile?: string;
    configFile?: string;
    buildCommand?: string;
    testCommand?: string;
  };
  dependencies?: string[];
  devDependencies?: string[];
  entryPoint?: string;
  suggestedPorts?: number[];
}
